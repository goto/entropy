package driver

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/modules/job/config"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
)

func (driver *Driver) refreshOutput(ctx context.Context, r resource.Resource, config config.Config, output Output, out kubernetes.Output) (json.RawMessage, error) {
	return nil, nil
}
