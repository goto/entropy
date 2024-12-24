package firehose

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

func TestFirehoseDriver_Plan_create(t *testing.T) {
	t.Parallel()

	table := []struct {
		title   string
		exr     module.ExpandedResource
		act     module.ActionRequest
		want    *resource.Resource
		wantErr error
	}{
		// create action tests
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
			title: "Create_LongName",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:ABCDEFGHIJKLMNOPQRSTUVWXYZ:abcdefghijklmnopqrstuvwxyz",
					Kind:    "firehose",
					Name:    "abcdefghijklmnopqrstuvwxyz",
					Project: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.CreateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 1,
					"env_variables": map[string]string{
						"SINK_TYPE":                "LOG",
						"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
						"SOURCE_KAFKA_BROKERS":     "localhost:9092",
						"SOURCE_KAFKA_TOPIC":       "foo-log",
					},
				}),
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:ABCDEFGHIJKLMNOPQRSTUVWXYZ:abcdefghijklmnopqrstuvwxyz",
				Kind:    "firehose",
				Name:    "abcdefghijklmnopqrstuvwxyz",
				Project: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"stopped":       false,
						"replicas":      1,
						"namespace":     "firehose",
						"deployment_id": "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-3801d0-firehose",
						"chart_values": map[string]string{
							"chart_version":     "0.1.3",
							"image_repository":  "gotocompany/firehose",
							"image_pull_policy": "IfNotPresent",
							"image_tag":         "latest",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "LOG",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-3801d0-firehose-1",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-3801d0-firehose",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseCreate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
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
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.CreateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 1,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "LOG",
						"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
						"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
						"SOURCE_KAFKA_BROKERS":           "localhost:9092",
						"SOURCE_KAFKA_TOPIC":             "foo-log",
					},
				}),
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:foo:fh1",
				Kind:    "firehose",
				Name:    "fh1",
				Project: "foo",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"stopped":       false,
						"replicas":      1,
						"namespace":     "firehose",
						"deployment_id": "foo-fh1-firehose",
						"chart_values": map[string]string{

							"chart_version":     "0.1.3",
							"image_repository":  "gotocompany/firehose",
							"image_pull_policy": "IfNotPresent",
							"image_tag":         "latest",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "LOG",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "foo-fh1-firehose",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseCreate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
		{
			title: "Create_ValidRequest_Bigquery",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.CreateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 1,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "BIGQUERY",
						"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
						"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
						"SOURCE_KAFKA_BROKERS":           "localhost:9092",
						"SOURCE_KAFKA_TOPIC":             "foo-log",
					},
				}),
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:foo:fh1",
				Kind:    "firehose",
				Name:    "fh1",
				Project: "foo",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"stopped":       false,
						"replicas":      1,
						"namespace":     "bigquery-firehose",
						"deployment_id": "foo-fh1-firehose",
						"chart_values": map[string]string{

							"chart_version":     "0.1.3",
							"image_repository":  "gotocompany/firehose",
							"image_pull_policy": "IfNotPresent",
							"image_tag":         "latest",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "BIGQUERY",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "bigquery-firehose",
						ReleaseName: "foo-fh1-firehose",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseCreate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
		{
			// if kube resource has namespace key, all resources will be deployed to that namespace value
			title: "Create_ValidRequest_OverrideNamespace",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind: "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{
							Configs: kube.Config{
								Namespace: "override-namespace",
							},
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.CreateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 1,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "LOG",
						"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
						"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
						"SOURCE_KAFKA_BROKERS":           "localhost:9092",
						"SOURCE_KAFKA_TOPIC":             "foo-log",
					},
				}),
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:foo:fh1",
				Kind:    "firehose",
				Name:    "fh1",
				Project: "foo",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"stopped":       false,
						"replicas":      1,
						"namespace":     "override-namespace",
						"deployment_id": "foo-fh1-firehose",
						"chart_values": map[string]string{

							"chart_version":     "0.1.3",
							"image_repository":  "gotocompany/firehose",
							"image_pull_policy": "IfNotPresent",
							"image_tag":         "latest",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "LOG",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "override-namespace",
						ReleaseName: "foo-fh1-firehose",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseCreate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			dr := &firehoseDriver{
				conf:    defaultDriverConf,
				timeNow: func() time.Time { return frozenTime },
			}

			dr.conf.Namespace = map[string]string{
				defaultKey: "firehose",
				"BIGQUERY": "bigquery-firehose",
			}

			got, err := dr.Plan(context.Background(), tt.exr, tt.act)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.True(t, errors.Is(err, tt.wantErr), "wantErr=%v\ngotErr=%v", tt.wantErr, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, got)

				wantJSON := string(modules.MustJSON(tt.want))
				gotJSON := string(modules.MustJSON(got))
				assert.JSONEq(t, wantJSON, gotJSON)
			}
		})
	}
}
