package firehose

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var firehoseKedaTriggerRefreshCount = mustFirehoseCounter(
	"entropy.firehose.keda.kafka_trigger.refresh.count",
	"KEDA kafka trigger metadata refreshes during firehose plan",
)

func recordKedaKafkaTriggerRefresh(urn string, authRefSet bool) {
	firehoseKedaTriggerRefreshCount.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("resource", urn),
		attribute.Bool("authentication_ref_set", authRefSet),
	))
}

func observeKedaTriggerRefreshSkipped(resourceURN, reason string) {
	zap.L().Debug("firehose keda kafka trigger refresh skipped",
		zap.String("resource", resourceURN),
		zap.String("reason", reason),
	)
}

func observeKedaKafkaTriggersRefreshed(resourceURN string, keda *Keda, cfg Config) {
	if keda == nil || len(keda.Triggers) == 0 {
		observeKedaTriggerRefreshSkipped(resourceURN, "no_triggers")
		return
	}

	kafkaTriggers := 0
	for triggerName, trigger := range keda.Triggers {
		if trigger.Type != KAFKA {
			continue
		}
		kafkaTriggers++

		authName := trigger.AuthenticationRef.Name
		recordKedaKafkaTriggerRefresh(resourceURN, authName != "")

		fields := []zap.Field{
			zap.String("resource", resourceURN),
			zap.String("trigger", triggerName),
			zap.String("source_stream", cfg.StreamName),
			zap.Bool("stream_security_enabled", cfg.StreamSecurityEnabled),
			zap.Bool("acl_chart_values", cfg.ACL != nil),
			zap.String("consumer_group", trigger.Metadata[KedaKafkaMetadataConsumerGroupKey]),
			zap.String("topic", trigger.Metadata[KedaKafkaMetadataTopicKey]),
			zap.String("bootstrap_servers", trigger.Metadata[KedaKafkaMetadataBootstrapServersKey]),
			zap.String("tls", trigger.Metadata[KedaKafkaMetadataTLSKey]),
			zap.String("authentication_ref", authName),
		}
		zap.L().Info("firehose keda kafka trigger metadata refreshed", fields...)

		if cfg.StreamSecurityEnabled && authName == "" {
			zap.L().Warn("firehose keda kafka trigger missing authenticationRef for ACL stream",
				zap.String("resource", resourceURN),
				zap.String("trigger", triggerName),
				zap.String("source_stream", cfg.StreamName),
				zap.Bool("acl_chart_values", cfg.ACL != nil),
			)
		}
	}

	if kafkaTriggers == 0 {
		observeKedaTriggerRefreshSkipped(resourceURN, "no_kafka_triggers")
	}
}
