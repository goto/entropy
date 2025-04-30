package firehose

import (
	"os"
	"testing"

	"encoding/json"
)

func BenchmarkDriverFactory(b *testing.B) {
	b.SetParallelism(10000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			configFile, err := os.ReadFile("./test/module-config.json")
			if err != nil {
				b.Fatalf("Failed to read file: %v", err)
			}

			config := json.RawMessage(configFile)

			_, _ = Module.DriverFactory(config)
		}
	})
}
