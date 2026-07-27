package module

import (
	"encoding/json"
	"testing"

	"github.com/goto/entropy/pkg/errors"
)

func TestValidateSensitiveConfigs(t *testing.T) {
	tests := []struct {
		name    string
		configs string
		wantErr bool
	}{
		{name: "no sensitive_configs", configs: `{"namespace":{"default":"x"}}`},
		{name: "valid paths", configs: `{"sensitive_configs":["env_variables.PWD","env_variables.*","gcs_cred"]}`},
		{name: "empty string", configs: ``},
		{name: "empty segment", configs: `{"sensitive_configs":["a..b"]}`, wantErr: true},
		{name: "wildcard not last", configs: `{"sensitive_configs":["a.*.b"]}`, wantErr: true},
		{name: "empty path", configs: `{"sensitive_configs":[""]}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSensitiveConfigs(json.RawMessage(tt.configs))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, errors.ErrInvalid) {
					t.Errorf("expected ErrInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
