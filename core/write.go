package core

import (
	"context"
	"fmt"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type Options struct {
	DryRun bool
}

func WithDryRun(dryRun bool) Options {
	return Options{DryRun: dryRun}
}

func (svc *Service) CreateResource(ctx context.Context, res resource.Resource, resourceOpts ...Options) (*resource.Resource, error) {
	if err := res.Validate(true); err != nil {
		return nil, err
	}

	act := module.ActionRequest{
		Name:   module.CreateAction,
		Params: res.Spec.Configs,
		Labels: res.Labels,
		UserID: res.CreatedBy,
	}
	res.Spec.Configs = nil

	dryRun := false
	for _, opt := range resourceOpts {
		dryRun = opt.DryRun
	}

	return svc.execAction(ctx, res, act, dryRun)
}

func (svc *Service) UpdateResource(ctx context.Context, urn string, req resource.UpdateRequest, resourceOpts ...Options) (*resource.Resource, error) {
	if len(req.Spec.Dependencies) != 0 {
		return nil, errors.ErrUnsupported.WithMsgf("updating dependencies is not supported")
	} else if len(req.Spec.Configs) == 0 {
		return nil, errors.ErrInvalid.WithMsgf("no config is being updated, nothing to do")
	}

	return svc.ApplyAction(ctx, urn, module.ActionRequest{
		Name:   module.UpdateAction,
		Params: req.Spec.Configs,
		Labels: req.Labels,
		UserID: req.UserID,
	}, resourceOpts...)
}

func (svc *Service) DeleteResource(ctx context.Context, urn string) error {
	_, actionErr := svc.ApplyAction(ctx, urn, module.ActionRequest{
		Name: module.DeleteAction,
	}, WithDryRun(false))
	return actionErr
}

func (svc *Service) ApplyAction(ctx context.Context, urn string, act module.ActionRequest, resourceOpts ...Options) (*resource.Resource, error) {
	res, err := svc.GetResource(ctx, urn)
	if err != nil {
		return nil, err
	} else if !res.State.IsTerminal() {
		return nil, errors.ErrInvalid.
			WithMsgf("cannot perform '%s' on resource in '%s'", act.Name, res.State.Status)
	}

	dryRun := false
	for _, opt := range resourceOpts {
		dryRun = opt.DryRun
	}

	return svc.execAction(ctx, *res, act, dryRun)
}

func (svc *Service) execAction(ctx context.Context, res resource.Resource, act module.ActionRequest, dryRun bool) (*resource.Resource, error) {
	logEntry := zap.L().With(
		zap.String("resource_urn", res.URN),
		zap.String("resource_status", res.State.Status),
		zap.Int("retries", res.State.SyncResult.Retries),
		zap.String("last_err", res.State.SyncResult.LastError),
	)

	planned, err := svc.planChange(ctx, res, act)
	if err != nil {
		return nil, err
	}

	if isCreate(act.Name) {
		planned.CreatedAt = svc.clock()
		planned.UpdatedAt = planned.CreatedAt
		planned.CreatedBy = act.UserID
		planned.UpdatedBy = act.UserID
	} else {
		planned.CreatedAt = res.CreatedAt
		planned.UpdatedAt = svc.clock()
		planned.UpdatedBy = act.UserID
	}

	reason := fmt.Sprintf("action:%s", act.Name)

	if !dryRun {
		if err := svc.upsert(ctx, *planned, isCreate(act.Name), true, reason); err != nil {
			return nil, err
		}
	}

	meter := telemetry.GetMeter(svc.serviceName)
	pendingCounter, err := setupCounter(meter, PendingCounter)
	if err != nil {
		return nil, err
	}

	// Increment the pending counter.
	logEntry.Info("Incrementing pending counter")
	pendingCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resource", res.URN)))

	return planned, nil
}

func (svc *Service) planChange(ctx context.Context, res resource.Resource, act module.ActionRequest) (*resource.Resource, error) {
	modSpec, err := svc.generateModuleSpec(ctx, res)
	if err != nil {
		return nil, err
	}

	planned, err := svc.moduleSvc.PlanAction(ctx, *modSpec, act)
	if err != nil {
		if errors.Is(err, errors.ErrInvalid) {
			return nil, err
		}
		return nil, errors.ErrInternal.WithMsgf("plan() failed").WithCausef(err.Error())
	}

	planned.Labels = mergeLabels(res.Labels, act.Labels)
	if err := planned.Validate(isCreate(act.Name)); err != nil {
		return nil, err
	}
	return planned, nil
}

func (svc *Service) upsert(ctx context.Context, res resource.Resource, isCreate bool, saveRevision bool, reason string) error {
	var err error
	if isCreate {
		err = svc.store.Create(ctx, res)
	} else {
		err = svc.store.Update(ctx, res, saveRevision, reason)
	}

	if err != nil {
		if isCreate && errors.Is(err, errors.ErrConflict) {
			return errors.ErrConflict.WithMsgf("resource with urn '%s' already exists", res.URN)
		} else if !isCreate && errors.Is(err, errors.ErrNotFound) {
			return errors.ErrNotFound.WithMsgf("resource with urn '%s' does not exist", res.URN)
		}
		return errors.ErrInternal.WithCausef(err.Error())
	}

	return nil
}

func isCreate(actionName string) bool {
	return actionName == module.CreateAction
}

func mergeLabels(m1, m2 map[string]string) map[string]string {
	if m1 == nil && m2 == nil {
		return nil
	}
	res := map[string]string{}
	for k, v := range m1 {
		res[k] = v
	}
	for k, v := range m2 {
		res[k] = v
	}

	// clean the empty values.
	for k, v := range res {
		if v == "" {
			delete(res, k)
		}
	}
	return res
}
