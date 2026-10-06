package firehose

import (
	"context"

	kafkamod "github.com/goto/entropy/modules/kafka"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	dlqSecurityModeSameSource  = "same_source"
	dlqSecurityModeDLQStream = "dlq_stream"
	dlqSecurityModePlaintext   = "plaintext"
	dlqSecurityModeUnresolved  = "unresolved"
)

var firehoseKafkaDLQSecurityCount = mustFirehoseCounter(
	"entropy.firehose.dlq.kafka_security.wiring.count",
	"Kafka DLQ producer ACL wiring decisions during firehose plan",
)

func recordKafkaDLQSecurityWiring(urn, mode, securityProtocol string) {
	firehoseKafkaDLQSecurityCount.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("resource", urn),
		attribute.String("mode", mode),
		attribute.String("security_protocol", securityProtocol),
	))
}

func observeKafkaSourceSecurityWired(urn, streamName string, sp *kafkamod.SecurityProfile) {
	if !hasSecurityProfile(sp) {
		return
	}
	zap.L().Info("firehose kafka source security wired",
		zap.String("resource", urn),
		zap.String("source_stream", streamName),
		zap.String("security_protocol", sp.SecurityProtocol),
		zap.String("sasl_mechanism", sp.SaslMechanism),
	)
}

func observeKafkaDLQSecurityWiring(urn, mode, sourceStream, dlqStreamURN, dlqResource string, sp *kafkamod.SecurityProfile, aclChartValuesForDLQ bool) {
	protocol := ""
	if sp != nil {
		protocol = sp.SecurityProtocol
	}
	recordKafkaDLQSecurityWiring(urn, mode, protocol)

	fields := []zap.Field{
		zap.String("resource", urn),
		zap.String("mode", mode),
		zap.String("source_stream", sourceStream),
		zap.String("dlq_stream_urn", dlqStreamURN),
		zap.String("dlq_kafka_resource", dlqResource),
		zap.String("security_protocol", protocol),
		zap.Bool("dlq_acl_chart_values", aclChartValuesForDLQ),
	}
	if sp != nil && sp.SaslMechanism != "" {
		fields = append(fields, zap.String("sasl_mechanism", sp.SaslMechanism))
	}

	switch mode {
	case dlqSecurityModePlaintext, dlqSecurityModeUnresolved:
		zap.L().Debug("firehose kafka DLQ security wiring skipped", fields...)
	default:
		zap.L().Info("firehose kafka DLQ security wired", fields...)
	}
}
