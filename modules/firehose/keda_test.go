package firehose

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeda_Validate(t *testing.T) {
	tests := []struct {
		name    string
		keda    Keda
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Empty config should error",
			keda:    Keda{},
			wantErr: true,
			errMsg:  "min_replicas and max_replicas must be set when autoscaler is enabled",
		},
		{
			name: "Invalid min replicas",
			keda: Keda{
				MinReplicas: -1,
				MaxReplicas: 5,
				Triggers: map[string]Trigger{
					"test": {Type: KAFKA},
				},
			},
			wantErr: true,
			errMsg:  "min_replicas must be greater than or equal to 0",
		},
		{
			name: "Invalid max replicas",
			keda: Keda{
				MinReplicas: 1,
				MaxReplicas: 0,
				Triggers: map[string]Trigger{
					"test": {Type: KAFKA},
				},
			},
			wantErr: true,
			errMsg:  "max_replicas must be greater than or equal to 1",
		},
		{
			name: "Min greater than max",
			keda: Keda{
				MinReplicas: 5,
				MaxReplicas: 3,
				Triggers: map[string]Trigger{
					"test": {Type: KAFKA},
				},
			},
			wantErr: true,
			errMsg:  "min_replicas must be less than or equal to max_replicas",
		},
		{
			name: "No triggers defined",
			keda: Keda{
				MinReplicas: 1,
				MaxReplicas: 3,
			},
			wantErr: true,
			errMsg:  "at least one trigger must be defined when autoscaler is enabled",
		},
		{
			name: "Valid config",
			keda: Keda{
				MinReplicas: 1,
				MaxReplicas: 5,
				Triggers: map[string]Trigger{
					"test": {Type: KAFKA},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.keda.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeda_PauseResume(t *testing.T) {
	tests := []struct {
		name     string
		replica  []int
		wantKeda Keda
	}{
		{
			name:    "Pause without replica",
			replica: []int{},
			wantKeda: Keda{
				Paused: true,
			},
		},
		{
			name:    "Pause with replica",
			replica: []int{3},
			wantKeda: Keda{
				PausedWithReplica: true,
				PausedReplica:     3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &Keda{}
			k.Pause(tt.replica...)
			assert.Equal(t, tt.wantKeda.Paused, k.Paused)
			assert.Equal(t, tt.wantKeda.PausedWithReplica, k.PausedWithReplica)
			assert.Equal(t, tt.wantKeda.PausedReplica, k.PausedReplica)

			k.Resume()
			assert.False(t, k.Paused)
			assert.False(t, k.PausedWithReplica)
		})
	}
}
