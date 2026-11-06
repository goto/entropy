package webui_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/addons/webui"
	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

const parentURN = "orn:entropy:dagger:team-a:orders-agg"

func TestDriver_Apply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		proxies fakeProxies
		wantOut string
		wantErr bool
	}{
		{
			name: "RecordsURLAndVersion",
			proxies: fakeProxies{handler: http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/overview" {
					wr.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = wr.Write([]byte(`{"flink-version":"1.17.1","taskmanagers":1}`))
			})},
			wantOut: `{"url":"/api/v1beta1/resources/` + parentURN + `/webui/","flink_version":"1.17.1"}`,
		},
		{
			name: "ProbeNon2xx",
			proxies: fakeProxies{handler: http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
				wr.WriteHeader(http.StatusBadGateway)
			})},
			wantErr: true,
		},
		{
			name:    "ProxyError",
			proxies: fakeProxies{err: errors.ErrUnsupported},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := webui.New(tt.proxies).Apply(context.Background(), addon.ParentContext{URN: parentURN}, nil, json.RawMessage(`{}`))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantOut, string(out))
		})
	}
}

func TestDriver_Contract(t *testing.T) {
	t.Parallel()

	drv := webui.New(fakeProxies{})
	assert.NoError(t, drv.Validate(json.RawMessage(`{}`)))
	assert.ErrorIs(t, drv.Validate(json.RawMessage(`{"port":8081}`)), errors.ErrInvalid)
	assert.True(t, drv.RequiresParentCompleted())
	assert.False(t, drv.RequiresConfig())
	assert.Empty(t, drv.SensitiveConfigKeys())
	assert.NoError(t, drv.Delete(context.Background(), addon.ParentContext{URN: parentURN}, nil, nil))
}

type fakeProxies struct {
	handler http.Handler
	err     error
}

func (f fakeProxies) GetWebUIProxy(_ context.Context, urn string) (http.Handler, error) {
	if urn != parentURN {
		return nil, errors.ErrNotFound
	}
	return f.handler, f.err
}
