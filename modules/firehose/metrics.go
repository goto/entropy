package firehose

import (
	"context"
	"strconv"
	"strings"

	"github.com/goto/entropy/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var firehoseDLQValidationCount = mustFirehoseCounter(
	"entropy.firehose.dlq.validation.count",
	"Count of Kafka DLQ env validation during firehose helm render",
)

func mustFirehoseCounter(name, description string) metric.Int64Counter {
	c, err := telemetry.GetMeter("entropy/firehose").Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}
	return c
}

func recordFirehoseDLQValidation(urn, result string) {
	firehoseDLQValidationCount.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("result", result),
		attribute.String("resource", urn),
	))
}

func recordFirehoseDLQValidationIfEnabled(urn string, envVars map[string]string) {
	enabled, _ := strconv.ParseBool(envVars[confDLQSinkEnable])
	if !enabled || !strings.EqualFold(strings.TrimSpace(envVars[confDLQWriterType]), dlqWriterTypeKafka) {
		return
	}
	recordFirehoseDLQValidation(urn, "ok")
}
