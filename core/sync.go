package core

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

type SyncStatus string

const (
	PendingCounter   SyncStatus = "pending"
	CompletedCounter SyncStatus = "completed"
	ErrorCounter     SyncStatus = "error"
	RetryCounter     SyncStatus = "retry"
)

// RunSyncer runs the syncer thread that keeps performing resource-sync at
// regular intervals.
func (svc *Service) RunSyncer(ctx context.Context, workerCount int, interval time.Duration, scope map[string][]string, eg *errgroup.Group) {
	for i := 0; i < workerCount; i++ {
		eg.Go(func() error {
			tick := time.NewTimer(interval)
			defer tick.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					tick.Reset(interval)

					err := svc.store.SyncOne(ctx, scope, svc.handleSync)
					if err != nil {
						zap.L().Warn("SyncOne() failed", zap.Error(err))
					}
				}
			}
		})
	}
}

func (svc *Service) handleSync(ctx context.Context, res resource.Resource) (*resource.Resource, error) {
	logEntry := zap.L().With(
		zap.String("resource_urn", res.URN),
		zap.String("resource_status", res.State.Status),
		zap.Int("retries", res.State.SyncResult.Retries),
		zap.String("last_err", res.State.SyncResult.LastError),
	)

	meter := telemetry.GetMeter(svc.serviceName)
	retryCounter, err := setupCounter(meter, RetryCounter)
	if err != nil {
		return nil, err
	}
	errorCounter, err := setupCounter(meter, ErrorCounter)
	if err != nil {
		return nil, err
	}
	completedCounter, err := setupCounter(meter, CompletedCounter)
	if err != nil {
		return nil, err
	}

	modSpec, err := svc.generateModuleSpec(ctx, res)
	if err != nil {
		logEntry.Error("SyncOne() failed", zap.Error(err))
		return nil, err
	}

	newState, err := svc.moduleSvc.SyncState(ctx, *modSpec)
	if err != nil {
		logEntry.Error("SyncOne() failed", zap.Error(err))

		res.State.SyncResult.LastError = err.Error()
		res.State.SyncResult.Retries++

		// Increment the retry counter.
		logEntry.Info("Incrementing retry counter")
		retryCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resource", res.URN)))

		if errors.Is(err, errors.ErrInvalid) {
			// ErrInvalid is expected to be returned when config is invalid.
			// There is no point in retrying in this case.
			res.State.Status = resource.StatusError
			res.State.NextSyncAt = nil

			// Increment the error counter.
			logEntry.Info("Incrementing error counter")
			errorCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resource", res.URN)))
		} else if svc.maxSyncRetries > 0 && res.State.SyncResult.Retries >= svc.maxSyncRetries {
			// Some other error occurred and no more retries remaining.
			// move the resource to failure state.
			res.State.Status = resource.StatusError
			res.State.NextSyncAt = nil

			// Increment the error counter.
			logEntry.Info("Incrementing error counter")
			errorCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resource", res.URN)))
		} else {
			// Some other error occurred and we still have remaining retries.
			// need to backoff and retry in some time.
			tryAgainAt := svc.clock().Add(svc.syncBackoff)
			res.State.NextSyncAt = &tryAgainAt
		}
	} else {
		res.State.SyncResult.Retries = 0
		res.State.SyncResult.LastError = ""
		res.UpdatedAt = svc.clock()
		res.State = *newState

		// Increment the completed counter.
		logEntry.Info("Incrementing completed counter")
		completedCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resource", res.URN)))

		logEntry.Info("SyncOne() finished",
			zap.String("final_status", res.State.Status),
			zap.Timep("next_sync", res.State.NextSyncAt),
		)
	}

	return &res, nil
}

func setupCounter(meter metric.Meter, countername SyncStatus) (metric.Int64Counter, error) {
	return meter.Int64Counter(
		fmt.Sprintf("%s_counter", countername),
		metric.WithDescription(fmt.Sprintf("Total number of %s performed", countername)),
		metric.WithUnit("1"),
	)
}
