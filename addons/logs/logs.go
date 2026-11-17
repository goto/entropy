// Package logs implements the "logs" add-on, which registers the workload of a
// parent resource with an external logging API configured per project.
package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

const maxErrorBody = 512

// Spec is the spec of a logs add-on.
type Spec struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// Config is the project addon-config of the logs add-on.
type Config struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

// Output is the output of a successful apply.
type Output struct {
	RegistrationID string `json:"registration_id"`
}

type registration struct {
	ParentURN string `json:"parent_urn"`
	Namespace string `json:"namespace"`
	Spec      Spec   `json:"spec"`
}

type parentOutput struct {
	Namespace string `json:"namespace"`
}

type Driver struct {
	client *http.Client
}

var _ addon.Driver = (*Driver)(nil)

func New(client *http.Client) *Driver {
	return &Driver{client: client}
}

func (*Driver) RequiresParentCompleted() bool { return true }

func (*Driver) RequiresConfig() bool { return true }

func (*Driver) SensitiveConfigKeys() []string { return []string{"token"} }

func (*Driver) Validate(spec json.RawMessage) error {
	_, err := readSpec(spec)
	return err
}

func (d *Driver) Apply(ctx context.Context, parent addon.ParentContext, config, spec json.RawMessage) (json.RawMessage, error) {
	conf, err := readConfig(config)
	if err != nil {
		return nil, err
	}

	s, err := readSpec(spec)
	if err != nil {
		return nil, err
	}

	var out parentOutput
	if len(parent.Output) > 0 {
		if err := json.Unmarshal(parent.Output, &out); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("parent output is not valid JSON")
		}
	}
	if out.Namespace == "" {
		return nil, errors.ErrInvalid.WithMsgf("parent output has no namespace")
	}

	body, err := json.Marshal(registration{ParentURN: parent.URN, Namespace: out.Namespace, Spec: s})
	if err != nil {
		return nil, err
	}

	resp, err := d.do(ctx, http.MethodPut, conf, "/registrations", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, statusErr(resp)
	}

	var result Output
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.RegistrationID == "" {
		result.RegistrationID = parent.URN
	}
	return json.Marshal(result)
}

func (d *Driver) Delete(ctx context.Context, parent addon.ParentContext, config, _ json.RawMessage) error {
	conf, err := readConfig(config)
	if err != nil {
		return err
	}

	resp, err := d.do(ctx, http.MethodDelete, conf, "/registrations/"+url.PathEscape(parent.URN), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return statusErr(resp)
	}
	return nil
}

func (d *Driver) do(ctx context.Context, method string, conf Config, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(conf.Endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if conf.Token != "" {
		req.Header.Set("Authorization", "Bearer "+conf.Token)
	}
	return d.client.Do(req)
}

func statusErr(resp *http.Response) error {
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return fmt.Errorf("logging api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
}

func readSpec(spec json.RawMessage) (Spec, error) {
	var s Spec
	dec := json.NewDecoder(bytes.NewReader(spec))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Spec{}, errors.ErrInvalid.WithMsgf("invalid logs add-on spec: %s", err.Error())
	}
	return s, nil
}

func readConfig(config json.RawMessage) (Config, error) {
	var c Config
	if err := json.Unmarshal(config, &c); err != nil {
		return Config{}, errors.ErrInvalid.WithMsgf("invalid logs addon-config: %s", err.Error())
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return Config{}, errors.ErrInvalid.WithMsgf("logs addon-config needs a valid endpoint")
	}
	return c, nil
}
