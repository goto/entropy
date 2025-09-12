package firehose

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutoscaler_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    *Autoscaler
		wantErr bool
	}{
		{
			name: "valid keda config",
			json: `{
				"enabled": true,
				"type": "keda",
				"spec": {
					"min_replicas": 1,
					"max_replicas": 10,
					"triggers": {
						"kafka-trigger": {
							"type": "kafka",
							"metadata": {
								"lag_threshold": "100"
							}
						}
					}
				}
			}`,
			want: &Autoscaler{
				Enabled: true,
				Type:    KEDA,
				Spec: &Keda{
					MinReplicas: 1,
					MaxReplicas: 10,
					Triggers: map[string]Trigger{
						"kafka-trigger": {
							Type: KAFKA,
							Metadata: map[string]string{
								"lag_threshold": "100",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid autoscaler type",
			json: `{
				"enabled": true,
				"type": "unsupported",
				"spec": {}
			}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name: "invalid keda spec",
			json: `{
				"enabled": true,
				"type": "keda",
				"spec": "invalid"
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Autoscaler
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.Enabled, got.Enabled)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Spec, got.Spec)
		})
	}
}

func TestAutoscaler_Validate(t *testing.T) {
	tests := []struct {
		name       string
		autoscaler *Autoscaler
		wantErr    bool
	}{
		{
			name: "valid config",
			autoscaler: &Autoscaler{
				Enabled: true,
				Type:    KEDA,
				Spec: &Keda{
					MinReplicas: 1,
					MaxReplicas: 10,
					Triggers: map[string]Trigger{
						"kafka-trigger": {
							Type: KAFKA,
							Metadata: map[string]string{
								"lag_threshold": "100",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "disabled autoscaler",
			autoscaler: &Autoscaler{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "enabled but no spec",
			autoscaler: &Autoscaler{
				Enabled: true,
				Type:    KEDA,
			},
			wantErr: true,
		},
		{
			name: "invalid config - min replicas greater than max",
			autoscaler: &Autoscaler{
				Enabled: true,
				Type:    KEDA,
				Spec: &Keda{
					MinReplicas: 10,
					MaxReplicas: 1,
					Triggers: map[string]Trigger{
						"kafka-trigger": {
							Type: KAFKA,
							Metadata: map[string]string{
								"lag_threshold": "100",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config - zero max replicas",
			autoscaler: &Autoscaler{
				Enabled: true,
				Type:    KEDA,
				Spec: &Keda{
					MinReplicas: 1,
					MaxReplicas: 0,
					Triggers: map[string]Trigger{
						"kafka-trigger": {
							Type: KAFKA,
							Metadata: map[string]string{
								"lag_threshold": "100",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.autoscaler.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
