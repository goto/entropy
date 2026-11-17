package logs_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/addons/logs"
	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

const parentURN = "orn:entropy:firehose:team-a:orders"

var parent = addon.ParentContext{
	URN:    parentURN,
	Kind:   "firehose",
	Output: json.RawMessage(`{"namespace":"team-a-orders"}`),
}

// testToken is a fake credential used only against the local test server.
const testToken = "test-token"

func TestDriver_Apply(t *testing.T) {
	t.Parallel()

	t.Run("RegistersWorkload", func(t *testing.T) {
		t.Parallel()

		var gotMethod, gotPath, gotAuth string
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			gotMethod, gotPath, gotAuth = req.Method, req.URL.Path, req.Header.Get("Authorization")
			_ = json.NewDecoder(req.Body).Decode(&gotBody)
			_, _ = wr.Write([]byte(`{"registration_id":"reg-1"}`))
		}))
		defer srv.Close()

		out, err := logs.New(srv.Client()).Apply(context.Background(), parent, config(srv.URL+"/"),
			json.RawMessage(`{"labels":{"app":"orders"}}`))
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, gotMethod)
		assert.Equal(t, "/registrations", gotPath)
		assert.Equal(t, "Bearer "+testToken, gotAuth)
		assert.Equal(t, map[string]any{
			"parent_urn": parentURN,
			"namespace":  "team-a-orders",
			"spec":       map[string]any{"labels": map[string]any{"app": "orders"}},
		}, gotBody)
		assert.JSONEq(t, `{"registration_id":"reg-1"}`, string(out))
	})

	t.Run("DefaultsRegistrationID", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
			wr.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		out, err := logs.New(srv.Client()).Apply(context.Background(), parent, config(srv.URL), json.RawMessage(`{}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"registration_id":"`+parentURN+`"}`, string(out))
	})

	t.Run("Non2xx", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
			http.Error(wr, "boom", http.StatusBadGateway)
		}))
		defer srv.Close()

		_, err := logs.New(srv.Client()).Apply(context.Background(), parent, config(srv.URL), json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "502")
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("MissingNamespace", func(t *testing.T) {
		t.Parallel()

		p := parent
		p.Output = json.RawMessage(`{}`)
		_, err := logs.New(http.DefaultClient).Apply(context.Background(), p, config("http://localhost"), json.RawMessage(`{}`))
		assert.ErrorIs(t, err, errors.ErrInvalid)
	})

	t.Run("InvalidEndpoint", func(t *testing.T) {
		t.Parallel()

		_, err := logs.New(http.DefaultClient).Apply(context.Background(), parent, json.RawMessage(`{"endpoint":"not a url"}`), json.RawMessage(`{}`))
		assert.ErrorIs(t, err, errors.ErrInvalid)
	})
}

func TestDriver_Delete(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				gotMethod, gotPath = req.Method, req.URL.EscapedPath()
				wr.WriteHeader(status)
			}))
			defer srv.Close()

			err := logs.New(srv.Client()).Delete(context.Background(), parent, config(srv.URL), json.RawMessage(`{}`))
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, gotMethod)
			assert.Equal(t, "/registrations/"+parentURN, gotPath)
		})
	}

	t.Run("ServerError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			_, _ = io.Copy(io.Discard, req.Body)
			wr.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		err := logs.New(srv.Client()).Delete(context.Background(), parent, config(srv.URL), json.RawMessage(`{}`))
		assert.Error(t, err)
	})
}

func TestDriver_Validate(t *testing.T) {
	t.Parallel()

	drv := logs.New(http.DefaultClient)
	assert.NoError(t, drv.Validate(json.RawMessage(`{"labels":{"a":"b"}}`)))
	assert.ErrorIs(t, drv.Validate(json.RawMessage(`{"unknown":1}`)), errors.ErrInvalid)
	assert.ErrorIs(t, drv.Validate(json.RawMessage(`{"labels":{"a":1}}`)), errors.ErrInvalid)
	assert.Equal(t, []string{"token"}, drv.SensitiveConfigKeys())
}

func config(endpoint string) json.RawMessage {
	raw, _ := json.Marshal(logs.Config{Endpoint: endpoint, Token: testToken})
	return raw
}
