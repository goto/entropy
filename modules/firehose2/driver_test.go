package firehose2

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

func TestFirehoseDriver_Plan(t *testing.T) {
	t.Parallel()

	table := []struct {
		title   string
		exr     module.ExpandedResource
		act     module.ActionRequest
		want    *module.Plan
		wantErr error
	}{
		// create action tests
		{
			title:   "UnsupportedAction",
			exr:     module.ExpandedResource{},
			act:     module.ActionRequest{Name: "FOO"},
			wantErr: errors.ErrInternal,
		},
		{
			title: "Create_InvalidParamsJSON",
			exr:   module.ExpandedResource{},
			act: module.ActionRequest{
				Name:   module.CreateAction,
				Params: []byte("{"),
			},
			wantErr: errors.ErrInvalid,
		},
		{
			title: "Create_InvalidParamsValue",
			exr:   module.ExpandedResource{},
			act: module.ActionRequest{
				Name:   module.CreateAction,
				Params: []byte("{}"),
			},
			wantErr: errors.ErrInvalid,
		},
		{
			title: "Create_ValidRequest",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
				},
			},
			act: module.ActionRequest{
				Name: module.CreateAction,
				Params: mustJSON(map[string]any{
					"replicas":             1,
					"kafka_topic":          "foo-bar",
					"kafka_broker_address": "localhost:9092",
				}),
			},
			want: &module.Plan{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: mustJSON(map[string]any{
							"replicas":             1,
							"kafka_topic":          "foo-bar",
							"kafka_consumer_id":    "foo-fh1-firehose-0001",
							"kafka_broker_address": "localhost:9092",
						}),
					},
					State: resource.State{
						Status: resource.StatusPending,
						Output: mustJSON(Output{
							Defaults: defaultDriverConf,
						}),
						ModuleData: mustJSON(transientData{
							PendingSteps: []string{stepReleaseCreate},
						}),
					},
				},
				Reason: "create_firehose",
			},
			wantErr: nil,
		},

		// update action tests
		{
			title: "Update_Valid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: mustJSON(Output{
							Namespace:   "foo",
							ReleaseName: "bar",
							Defaults:    defaultDriverConf,
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: mustJSON(map[string]any{
					"replicas":             1,
					"kafka_topic":          "foo-bar",
					"kafka_broker_address": "localhost:9092",
				}),
			},
			want: &module.Plan{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: mustJSON(map[string]any{
							"replicas":             1,
							"kafka_topic":          "foo-bar",
							"kafka_consumer_id":    "foo-fh1-firehose-0001",
							"kafka_broker_address": "localhost:9092",
						}),
					},
					State: resource.State{
						Status: resource.StatusPending,
						Output: mustJSON(Output{
							Defaults:    defaultDriverConf,
							Namespace:   "foo",
							ReleaseName: "bar",
						}),
						ModuleData: mustJSON(transientData{
							PendingSteps: []string{stepReleaseUpdate},
						}),
					},
				},
				Reason: "update_firehose",
			},
			wantErr: nil,
		},

		// reset action tests
		{
			title: "Reset_Valid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: mustJSON(map[string]any{
							"replicas":             1,
							"kafka_topic":          "foo-bar",
							"kafka_consumer_id":    "foo-fh1-firehose-0001",
							"kafka_broker_address": "localhost:9092",
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: mustJSON(Output{
							Defaults:    defaultDriverConf,
							Namespace:   "foo",
							ReleaseName: "bar",
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: ResetAction,
				Params: mustJSON(map[string]any{
					"to": "latest",
				}),
			},
			want: &module.Plan{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: mustJSON(map[string]any{
							"replicas":             1,
							"kafka_topic":          "foo-bar",
							"kafka_consumer_id":    "foo-fh1-firehose-0001",
							"kafka_broker_address": "localhost:9092",
						}),
					},
					State: resource.State{
						Status: resource.StatusPending,
						Output: mustJSON(Output{
							Defaults:    defaultDriverConf,
							Namespace:   "foo",
							ReleaseName: "bar",
						}),
						ModuleData: mustJSON(transientData{
							ResetTo: "latest",
							PendingSteps: []string{
								stepReleaseStop,
								stepConsumerReset,
								stepReleaseUpdate,
							},
						}),
					},
				},
				Reason: "reset_firehose",
			},
			wantErr: nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			dr := &firehoseDriver{
				conf: defaultDriverConf,
			}

			got, err := dr.Plan(context.Background(), tt.exr, tt.act)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.True(t, errors.Is(err, tt.wantErr), "wantErr=%v\ngotErr=%v", tt.wantErr, err)
			} else {
				wantJSON := string(mustJSON(tt.want))
				gotJSON := string(mustJSON(got))

				assert.NoError(t, err)
				require.NotNil(t, got)
				assert.JSONEq(t, wantJSON, gotJSON, cmp.Diff(wantJSON, gotJSON))
			}
		})
	}
}
