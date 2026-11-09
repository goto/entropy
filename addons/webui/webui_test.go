package webui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/goto/entropy/addons/webui"
	"github.com/goto/entropy/core/addon"
)

const parentURN = "orn:entropy:dagger:team-a:orders-agg"

func TestMeta_Output(t *testing.T) {
	t.Parallel()

	out := webui.New().Output(addon.ParentContext{URN: parentURN})
	assert.JSONEq(t, `{"url":"/api/v1beta1/resources/`+parentURN+`/webui/","target":"jobmanager","port":8081}`, string(out))
}
