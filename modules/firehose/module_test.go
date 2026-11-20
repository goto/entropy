package firehose

import (
	"os"
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkDriverFactory(b *testing.B) {
	desc := Module(nil)

	b.SetParallelism(10000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			configFile, err := os.ReadFile("./test/module-config.json")
			if err != nil {
				b.Fatalf("Failed to read file: %v", err)
			}

			config := json.RawMessage(configFile)

			_, _ = desc.DriverFactory(config)
		}
	})
}

// existing module configs with flat string labels must keep loading.
// note: decodes into a fresh driverConf instead of calling DriverFactory, which
// would decode into the maps shared with defaultDriverConf.
func TestDriverConf_FlatLabelsConfig(t *testing.T) {
	t.Parallel()

	configFile, err := os.ReadFile("./test/module-config.json")
	require.NoError(t, err)

	var conf driverConf
	require.NoError(t, json.Unmarshal(configFile, &conf))

	labels := conf.Labels
	assert.Equal(t, "dummy-app-id", labels.Common["app-id"])
	assert.Len(t, labels.Common, 17)
	assert.Empty(t, labels.BySink)
}
