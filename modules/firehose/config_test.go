package firehose

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/modules"
)

func Test_validateConfig_kafkaSink(t *testing.T) {
	t.Parallel()

	cfg := modules.MustJSON(map[string]any{
		"replicas": 1,
		"env_variables": map[string]string{
			"SINK_TYPE":                "KAFKA",
			"INPUT_SCHEMA_PROTO_CLASS": "com.foo.SourceMessage",
			"SOURCE_KAFKA_BROKERS":     "localhost:9092",
			"SOURCE_KAFKA_TOPIC":       "source-topic",
			"SINK_KAFKA_BROKERS":       "localhost:9093",
			"SINK_KAFKA_TOPIC":         "sink-topic",
			"SINK_KAFKA_PROTO_MESSAGE": "com.foo.SinkMessage",
			"SINK_KAFKA_PROTO_KEY":     "com.foo.SinkKey",
		},
	})

	err := validateConfig(cfg)
	require.NoError(t, err)
}

func Test_validateConfig_rejectsUnknownSinkType(t *testing.T) {
	t.Parallel()

	cfg := modules.MustJSON(map[string]any{
		"replicas": 1,
		"env_variables": map[string]string{
			"SINK_TYPE":                "UNKNOWN",
			"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
			"SOURCE_KAFKA_BROKERS":     "localhost:9092",
			"SOURCE_KAFKA_TOPIC":       "foo-log",
		},
	})

	err := validateConfig(cfg)
	require.Error(t, err)
}

func Test_safeReleaseName(t *testing.T) {
	t.Parallel()

	table := []struct {
		str  string
		want string
	}{
		{
			str:  "abcd-efgh",
			want: "abcd-efgh-firehose",
		},
		{
			str:  "abcd-efgh-firehose",
			want: "abcd-efgh-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghijklmnopqrstuvwxyz",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-3801d0-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghi---klmnopqrstuvwxyz",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghi-81c192-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghijklmnopqr-stuvwxyz1234567890",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-bac696-firehose",
		},
	}

	for i, tt := range table {
		t.Run(fmt.Sprintf("Case#%d", i), func(t *testing.T) {
			got := modules.SafeName(tt.str, "-firehose", helmReleaseNameMaxLength)
			assert.Equal(t, tt.want, got)
			assert.True(t, len(got) <= helmReleaseNameMaxLength, "release name has length %d", len(got))
		})
	}
}
