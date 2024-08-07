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
)

var frozenTime = time.Unix(1679668743, 0)

func TestFirehoseDriver_Plan(t *testing.T) {
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

		// update action tests
		{
			title: "Update_Valid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "gotocompany/firehose",
								"chart_version":     "1.0.0",
								"image_pull_policy": "",
								"image_tag":         "1.0.0",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                "LOG",
								"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
								"SOURCE_KAFKA_BROKERS":     "localhost:9092",
								"SOURCE_KAFKA_TOPIC":       "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind: "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{
							Tolerations: map[string][]kubernetes.Toleration{},
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 10,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "HTTP", // the change being applied
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
						"namespace":     "firehose",
						"stopped":       false,
						"replicas":      10,
						"deployment_id": "firehose-deployment-x",
						"chart_values": map[string]string{
							"image_repository":  "gotocompany/firehose",
							"chart_version":     "1.0.0",
							"image_pull_policy": "",
							"image_tag":         "1.0.0",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "HTTP",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseUpdate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
		// update override image repository
		{
			title: "Update_Override_Image_Repository",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "newrepo/firehose",
								"chart_version":     "1.0.0",
								"image_pull_policy": "",
								"image_tag":         "1.0.0",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                "LOG",
								"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
								"SOURCE_KAFKA_BROKERS":     "localhost:9092",
								"SOURCE_KAFKA_TOPIC":       "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind: "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{
							Tolerations: map[string][]kubernetes.Toleration{},
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 10,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "HTTP", // the change being applied
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
						"namespace":     "firehose",
						"stopped":       false,
						"replicas":      10,
						"deployment_id": "firehose-deployment-x",
						"chart_values": map[string]string{
							"image_repository":  "newrepo/firehose",
							"chart_version":     "1.0.0",
							"image_pull_policy": "",
							"image_tag":         "1.0.0",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "HTTP",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseUpdate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
		{
			title: "Update_Resource_&_Limits",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "gotocompany/firehose",
								"chart_version":     "1.0.0",
								"image_pull_policy": "",
								"image_tag":         "1.0.0",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                "LOG",
								"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
								"SOURCE_KAFKA_BROKERS":     "localhost:9092",
								"SOURCE_KAFKA_TOPIC":       "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 10,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "HTTP", // the change being applied
						"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
						"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
						"SOURCE_KAFKA_BROKERS":           "localhost:9092",
						"SOURCE_KAFKA_TOPIC":             "foo-log",
					},
					"limits": map[string]any{
						"cpu":    "500m",
						"memory": "2048Mi",
					},
					"requests": map[string]any{
						"cpu":    "400m",
						"memory": "1024Mi",
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
						"namespace":     "firehose",
						"stopped":       false,
						"replicas":      10,
						"deployment_id": "firehose-deployment-x",
						"chart_values": map[string]string{
							"image_repository":  "gotocompany/firehose",
							"chart_version":     "1.0.0",
							"image_pull_policy": "",
							"image_tag":         "1.0.0",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "HTTP",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"limits": map[string]any{
							"cpu":    "500m",
							"memory": "2048Mi",
						},
						"requests": map[string]any{
							"cpu":    "400m",
							"memory": "1024Mi",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseUpdate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},
		{
			title: "Update_Running_Firehose_Namespace",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"stopped":       false,
							"deployment_id": "firehose-deployment-x",
							"env_variables": map[string]string{
								"SINK_TYPE":                "LOG",
								"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
								"SOURCE_KAFKA_BROKERS":     "localhost:9092",
								"SOURCE_KAFKA_TOPIC":       "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 10,
					"stopped":  false,
					"env_variables": map[string]string{
						"SINK_TYPE":                      "BIGQUERY", // the change being applied
						"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
						"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
						"SOURCE_KAFKA_BROKERS":           "localhost:9092",
						"SOURCE_KAFKA_TOPIC":             "foo-log",
					},
				}),
			},
			want:    nil,
			wantErr: errors.ErrInvalid.WithCausef(errCauseInvalidNamespaceUpdate),
		},
		{
			title: "Update_Stopped_Firehose_Namespace",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"namespace":     "firehose",
							"replicas":      1,
							"stopped":       true,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "gotocompany/firehose",
								"chart_version":     "1.0.0",
								"image_pull_policy": "",
								"image_tag":         "1.0.0",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                "LOG",
								"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
								"SOURCE_KAFKA_BROKERS":     "localhost:9092",
								"SOURCE_KAFKA_TOPIC":       "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: module.UpdateAction,
				Params: modules.MustJSON(map[string]any{
					"replicas": 10,
					"stopped":  false, // shall allow starting at the time of update
					"env_variables": map[string]string{
						"SINK_TYPE":                      "BIGQUERY", // the change being applied
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
						"namespace":     "bigquery-firehose",
						"stopped":       false,
						"replicas":      10,
						"deployment_id": "firehose-deployment-x",
						"chart_values": map[string]string{
							"image_repository":  "gotocompany/firehose",
							"chart_version":     "1.0.0",
							"image_pull_policy": "",
							"image_tag":         "1.0.0",
						},
						"env_variables": map[string]string{
							"SINK_TYPE":                      "BIGQUERY",
							"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
							"SOURCE_KAFKA_BROKERS":           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":             "foo-log",
						},
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose", // this is updated when Output is triggered
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseUpdate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},

		// reset action tests
		{
			title: "Reset_InValid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"env_variables": map[string]string{
								"SINK_TYPE":                      "LOG",
								"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
								"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
								"SOURCE_KAFKA_BROKERS":           "localhost:9092",
								"SOURCE_KAFKA_TOPIC":             "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "foo",
							ReleaseName: "bar",
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: ResetAction,
				Params: modules.MustJSON(map[string]any{
					"to": "some_random",
				}),
			},
			wantErr: errors.ErrInvalid,
		},
		{
			title: "Reset_Valid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"env_variables": map[string]string{
								"SINK_TYPE":                                      "LOG",
								"INPUT_SCHEMA_PROTO_CLASS":                       "com.foo.Bar",
								"SOURCE_KAFKA_CONSUMER_GROUP_ID":                 "firehose-deployment-x-1",
								"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET": "latest",
								"SOURCE_KAFKA_BROKERS":                           "localhost:9092",
								"SOURCE_KAFKA_TOPIC":                             "foo-log",
							},
							"limits": map[string]any{
								"cpu":    "200m",
								"memory": "512Mi",
							},
							"requests": map[string]any{
								"cpu":    "200m",
								"memory": "512Mi",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name: ResetAction,
				Params: modules.MustJSON(map[string]any{
					"to": "earliest",
				}),
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:foo:fh1",
				Kind:    "firehose",
				Name:    "fh1",
				Project: "foo",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"namespace":     "firehose",
						"replicas":      1,
						"deployment_id": "firehose-deployment-x",
						"env_variables": map[string]string{
							"SINK_TYPE":                                      "LOG",
							"INPUT_SCHEMA_PROTO_CLASS":                       "com.foo.Bar",
							"SOURCE_KAFKA_CONSUMER_GROUP_ID":                 "firehose-deployment-x-2",
							"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET": "earliest",
							"SOURCE_KAFKA_BROKERS":                           "localhost:9092",
							"SOURCE_KAFKA_TOPIC":                             "foo-log",
						},
						"reset_offset": "earliest",
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "512Mi",
						},
						"stopped":        false,
						"init_container": map[string]interface{}{"args": interface{}(nil), "command": interface{}(nil), "enabled": false, "image_tag": "", "pull_policy": "", "repository": ""},
					}),
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: modules.MustJSON(Output{
						Namespace:   "firehose",
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{
							stepReleaseStop,
							stepReleaseUpdate,
						},
					}),
					NextSyncAt: &frozenTime,
				},
			},
		},

		// upgrade action tests
		{
			title: "Upgrade_Valid",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"stopped":       false,
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "gotocompany/firehose",
								"chart_version":     "0.1.0",
								"image_pull_policy": "IfNotPresent",
								"image_tag":         "latest",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                      "LOG",
								"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
								"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
								"SOURCE_KAFKA_BROKERS":           "localhost:9092",
								"SOURCE_KAFKA_TOPIC":             "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "firehose",
							ReleaseName: "bar",
						}),
					},
				},
				Dependencies: map[string]module.ResolvedDependency{
					"kube_cluster": {
						Kind:   "kubernetes",
						Output: modules.MustJSON(kubernetes.Output{}),
					},
				},
			},
			act: module.ActionRequest{
				Name: UpgradeAction,
			},
			want: &resource.Resource{
				URN:     "urn:goto:entropy:foo:fh1",
				Kind:    "firehose",
				Name:    "fh1",
				Project: "foo",
				Spec: resource.Spec{
					Configs: modules.MustJSON(map[string]any{
						"namespace":     "firehose",
						"stopped":       false,
						"replicas":      1,
						"deployment_id": "firehose-deployment-x",
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
						ReleaseName: "bar",
					}),
					ModuleData: modules.MustJSON(transientData{
						PendingSteps: []string{stepReleaseUpdate},
					}),
					NextSyncAt: &frozenTime,
				},
			},
			wantErr: nil,
		},

		// scale action tests
		{
			title: "Scale_Invalid_params",
			exr: module.ExpandedResource{
				Resource: resource.Resource{
					URN:     "urn:goto:entropy:foo:fh1",
					Kind:    "firehose",
					Name:    "fh1",
					Project: "foo",
					Spec: resource.Spec{
						Configs: modules.MustJSON(map[string]any{
							"replicas":      1,
							"deployment_id": "firehose-deployment-x",
							"chart_values": map[string]string{
								"image_repository":  "gotocompany/firehose",
								"chart_version":     "0.1.0",
								"image_pull_policy": "IfNotPresent",
								"image_tag":         "latest",
							},
							"env_variables": map[string]string{
								"SINK_TYPE":                      "LOG",
								"INPUT_SCHEMA_PROTO_CLASS":       "com.foo.Bar",
								"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
								"SOURCE_KAFKA_BROKERS":           "localhost:9092",
								"SOURCE_KAFKA_TOPIC":             "foo-log",
							},
						}),
					},
					State: resource.State{
						Status: resource.StatusCompleted,
						Output: modules.MustJSON(Output{
							Namespace:   "foo",
							ReleaseName: "bar",
						}),
					},
				},
			},
			act: module.ActionRequest{
				Name:   ScaleAction,
				Params: []byte("{}"),
			},
			wantErr: errors.ErrInvalid,
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

func TestGetNewConsumerGroupID(t *testing.T) {
	t.Parallel()

	table := []struct {
		title           string
		deploymentID    string
		consumerGroupID string
		want            string
		wantErr         error
	}{
		{
			title:           "invalid-group-id",
			consumerGroupID: "test-firehose-xyz",
			want:            "test-firehose-xyz-1",
			wantErr:         nil,
		},
		{
			title:           "valid-group-id",
			consumerGroupID: "test-firehose-0999",
			want:            "test-firehose-1000",
			wantErr:         nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			got, err := getNewConsumerGroupID(tt.consumerGroupID)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, "", got)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
