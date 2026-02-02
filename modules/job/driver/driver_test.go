package driver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/job/config"
	"github.com/goto/entropy/modules/kubernetes"
	kubejob "github.com/goto/entropy/pkg/kube/job"
	"github.com/goto/entropy/pkg/kube/pod"
)

func TestDriver(t *testing.T) {
	t.Parallel()

	table := []struct {
		title      string
		res        resource.Resource
		kubeOutput kubernetes.Output
		want       *kubejob.Job
		wantErr    error
	}{
		{
			title: "default flow",
			res: resource.Resource{
				URN:     "orn:entropy:job:test-1",
				Kind:    "job",
				Name:    "test-1",
				Project: "project-1",
				Labels: map[string]string{
					"team": "team-1",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				UpdatedBy: "john.doe@goto.com",
				CreatedBy: "john.doe@goto.com",
				Spec: resource.Spec{
					Configs: []byte(`{
                                     "env_variables": {
                                         "SINK_TYPE": "LOG",
                                         "INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
                                         "SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
                                         "SOURCE_KAFKA_BROKERS": "localhost:9092",
                                         "SOURCE_KAFKA_TOPIC": "foo-log"
                                     },
                                     "replicas": 1,
									 "namespace": "namespace-1"
                                 }`),
					Dependencies: map[string]string{},
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: nil,
				},
			},
			kubeOutput: kubernetes.Output{},
			want: &kubejob.Job{
				Name:      "project-1-test-1-job",
				Namespace: "namespace-1",
				Labels: map[string]string{
					"name":         "test-1",
					"orchestrator": "entropy",
				},
				Pod: &pod.Pod{
					Name: "project-1-test-1-job",
					Labels: map[string]string{
						"app": "project-1-test-1-job",
					},
				},
				Parallelism: func() *int32 { v := int32(1); return &v }(),
				BackOffList: func() *int32 { v := int32(0); return &v }(),
				TTLSeconds:  func() *int32 { v := int32(172800); return &v }(),
			},
			wantErr: nil,
		},
		{
			title: "with toleration and affinity",
			res: resource.Resource{
				URN:     "orn:entropy:job:test-1",
				Kind:    "job",
				Name:    "test-1",
				Project: "project-1",
				Labels: map[string]string{
					"team": "team-1",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				UpdatedBy: "john.doe@goto.com",
				CreatedBy: "john.doe@goto.com",
				Spec: resource.Spec{
					Configs: []byte(`{
                                     "env_variables": {
                                         "SINK_TYPE": "LOG",
                                         "INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
                                         "SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
                                         "SOURCE_KAFKA_BROKERS": "localhost:9092",
                                         "SOURCE_KAFKA_TOPIC": "foo-log"
                                     },
                                     "replicas": 1,
									 "namespace": "namespace-1"
                                 }`),
					Dependencies: map[string]string{},
				},
				State: resource.State{
					Status: resource.StatusPending,
					Output: nil,
				},
			},
			kubeOutput: kubernetes.Output{
				Affinities: map[string]pod.NodeAffinityMatchExpressions{
					"job": {
						RequiredDuringSchedulingIgnoredDuringExecution: []pod.Preference{
							{
								Key:      "name",
								Operator: "In",
								Values:   []string{"nodepool-1"},
							},
						},
					},
				},
				Tolerations: map[string][]pod.Toleration{
					"job": {
						{
							Key:      "key1",
							Operator: "Equal",
							Value:    "value1",
							Effect:   "NoSchedule",
						},
					},
					"firehose_BIGQUERY": {
						{
							Key:      "key2",
							Operator: "Equal",
							Value:    "value2",
							Effect:   "NoSchedule",
						},
					},
					"firehose_BLOB": {
						{
							Key:      "key3",
							Operator: "Equal",
							Value:    "value3",
							Effect:   "NoSchedule",
						},
					},
				},
			},
			want: &kubejob.Job{
				Name:      "project-1-test-1-job",
				Namespace: "namespace-1",
				Labels: map[string]string{
					"name":         "test-1",
					"orchestrator": "entropy",
				},
				Pod: &pod.Pod{
					Name: "project-1-test-1-job",
					Labels: map[string]string{
						"app": "project-1-test-1-job",
					},
					Tolerations: []pod.Toleration{
						{
							Key:      "key1",
							Operator: "Equal",
							Value:    "value1",
							Effect:   "NoSchedule",
						},
					},
					NodeAffinityMatchExpressions: &pod.NodeAffinityMatchExpressions{
						RequiredDuringSchedulingIgnoredDuringExecution: []pod.Preference{
							{
								Key:      "name",
								Operator: "In",
								Values:   []string{"nodepool-1"},
							},
						},
					},
				},
				Parallelism: func() *int32 { v := int32(1); return &v }(),
				BackOffList: func() *int32 { v := int32(0); return &v }(),
				TTLSeconds:  func() *int32 { v := int32(172800); return &v }(),
			},
			wantErr: nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			drv := &Driver{
				Conf: driverConf(),
			}

			conf, err := config.ReadConfig(tt.res, tt.res.Spec.Configs, drv.Conf)
			require.NoError(t, err)

			job := drv.getJob(tt.res, conf, tt.kubeOutput)

			require.NotNil(t, job)

			wantJSON := string(modules.MustJSON(tt.want))
			gotJSON := string(modules.MustJSON(job))
			assert.JSONEq(t, wantJSON, gotJSON)
		})
	}
}

func driverConf() config.DriverConf {
	return config.DriverConf{}
}
