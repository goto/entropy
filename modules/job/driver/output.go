package driver

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/job/config"
	"github.com/goto/entropy/modules/kubernetes"
)

func (*Driver) refreshOutput(context.Context, resource.Resource, config.Config, Output, kubernetes.Output) (json.RawMessage, error) {
	return json.RawMessage{}, nil
}
