// Package webui implements the "webui" meta add-on, which serves the web UI of
// a parent resource through the resource proxy. For dagger, that is the Flink
// UI of the JobManager: kube service <deployment_id>-rest on port 8081.
package webui

import (
	"encoding/json"
	"fmt"

	"github.com/goto/entropy/core/addon"
)

// jobManagerPort is the port of the Flink JobManager REST/UI service.
const jobManagerPort = 8081

// Output is the computed output of the add-on.
type Output struct {
	URL    string `json:"url"`
	Target string `json:"target"`
	Port   int    `json:"port"`
}

type Meta struct{}

var _ addon.Meta = Meta{}

func New() Meta { return Meta{} }

// Output records where the UI is served.
func (Meta) Output(parent addon.ParentContext) json.RawMessage {
	raw, _ := json.Marshal(Output{
		URL:    fmt.Sprintf("/api/v1beta1/resources/%s/webui/", parent.URN),
		Target: "jobmanager",
		Port:   jobManagerPort,
	})
	return raw
}
