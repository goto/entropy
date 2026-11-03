package dagger

import (
	"testing"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
)

func TestGetHelmReleaseHAURLs(t *testing.T) {
	const (
		tplURL        = `cosn://{{or .ha_override_bucket "gjk-i-acc-dagger-ha-cgk-s-01-1300000000"}}`
		defaultBucket = "cosn://gjk-i-acc-dagger-ha-cgk-s-01-1300000000"
		labelBucket   = "cosn://gjk-i-acc-dagger-ha-cgk-s-02-1300000000"
	)

	tests := []struct {
		name   string
		urls   [3]string // checkpoint, savepoint, k8s-ha
		labels map[string]string
		want   [3]string
	}{
		{
			name: "plain urls are kept",
			urls: [3]string{defaultBucket + "/checkpoints", defaultBucket + "/savepoints", defaultBucket + "/k8s-ha"},
			want: [3]string{defaultBucket + "/checkpoints", defaultBucket + "/savepoints", defaultBucket + "/k8s-ha"},
		},
		{
			name: "template without label uses the default bucket",
			urls: [3]string{tplURL + "/checkpoints", tplURL + "/savepoints", tplURL + "/k8s-ha"},
			want: [3]string{defaultBucket + "/checkpoints", defaultBucket + "/savepoints", defaultBucket + "/k8s-ha"},
		},
		{
			name:   "template with label uses the label's bucket",
			urls:   [3]string{tplURL + "/checkpoints", tplURL + "/savepoints", tplURL + "/k8s-ha"},
			labels: map[string]string{"ha_override_bucket": "gjk-i-acc-dagger-ha-cgk-s-02-1300000000"},
			want:   [3]string{labelBucket + "/checkpoints", labelBucket + "/savepoints", labelBucket + "/k8s-ha"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dd := &daggerDriver{conf: driverConf{}}
			rc, err := dd.getHelmRelease(resource.Resource{
				URN:     "orn:entropy:dagger:test:ha-test",
				Name:    "ha-test",
				Project: "test-project",
				Labels:  tt.labels,
			}, Config{
				ChartValues:         &ChartValues{},
				EnvVariables:        map[string]string{},
				DaggerCheckpointURL: tt.urls[0],
				DaggerSavepointURL:  tt.urls[1],
				DaggerK8sHAURL:      tt.urls[2],
			}, kubernetes.Output{})
			if err != nil {
				t.Fatalf("getHelmRelease returned error: %v", err)
			}
			got := [3]string{
				rc.Values["dagger_checkpoint_url"].(string),
				rc.Values["dagger_savepoint_url"].(string),
				rc.Values["dagger_k8s_ha_url"].(string),
			}
			if got != tt.want {
				t.Errorf("checkpoint, savepoint, k8s-ha urls = %q, want %q", got, tt.want)
			}
		})
	}
}
