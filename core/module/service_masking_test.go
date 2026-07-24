package module

import (
	"encoding/json"
	"testing"

	"github.com/goto/entropy/pkg/errors"
)

func TestValidateSensitiveConfig(t *testing.T) {
	tests := []struct {
		name    string
		configs string
		wantErr bool
	}{
		{name: "no sensitive_config", configs: `{"namespace":{"default":"x"}}`},
		{name: "valid paths", configs: `{"sensitive_config":["env_variables.PWD","env_variables.*","gcs_cred"]}`},
		{name: "empty string", configs: ``},
		{name: "empty segment", configs: `{"sensitive_config":["a..b"]}`, wantErr: true},
		{name: "wildcard not last", configs: `{"sensitive_config":["a.*.b"]}`, wantErr: true},
		{name: "empty path", configs: `{"sensitive_config":[""]}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSensitiveConfig(json.RawMessage(tt.configs))
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
