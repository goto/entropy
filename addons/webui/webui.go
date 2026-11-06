// Package webui implements the "webui" add-on, which enables the web UI proxy
// of a parent resource (e.g., the Flink UI of a dagger's JobManager).
package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

// probePath is the Flink REST cluster overview, served by the JobManager.
const probePath = "/overview"

// ProxyGetter returns the web UI proxy of a resource.
type ProxyGetter interface {
	GetWebUIProxy(ctx context.Context, urn string) (http.Handler, error)
}

// Output is the output of a successful apply.
type Output struct {
	URL          string `json:"url"`
	FlinkVersion string `json:"flink_version,omitempty"`
}

type overview struct {
	FlinkVersion string `json:"flink-version"`
}

type Driver struct {
	proxies ProxyGetter
}

var _ addon.Driver = (*Driver)(nil)

func New(proxies ProxyGetter) *Driver {
	return &Driver{proxies: proxies}
}

func (*Driver) RequiresParentCompleted() bool { return true }

func (*Driver) RequiresConfig() bool { return false }

func (*Driver) SensitiveConfigKeys() []string { return nil }

func (*Driver) Validate(spec json.RawMessage) error {
	var s struct{}
	dec := json.NewDecoder(bytes.NewReader(spec))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return errors.ErrInvalid.WithMsgf("invalid webui add-on spec: %s", err.Error())
	}
	return nil
}

// Apply checks that the UI is reachable through the proxy.
func (d *Driver) Apply(ctx context.Context, parent addon.ParentContext, _, _ json.RawMessage) (json.RawMessage, error) {
	proxy, err := d.proxies.GetWebUIProxy(ctx, parent.URN)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probePath, nil)
	if err != nil {
		return nil, err
	}
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code/100 != 2 {
		return nil, fmt.Errorf("web ui probe %s returned %d", probePath, rec.Code)
	}

	var ov overview
	_ = json.Unmarshal(rec.Body.Bytes(), &ov)

	return json.Marshal(Output{
		URL:          fmt.Sprintf("/api/v1beta1/resources/%s/webui/", parent.URN),
		FlinkVersion: ov.FlinkVersion,
	})
}

// Delete is a no-op: the UI is gated by the presence of the add-on.
func (*Driver) Delete(context.Context, addon.ParentContext, json.RawMessage, json.RawMessage) error {
	return nil
}
