package flink

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
)

type flinkDriver struct {
	conf driverConf
}

type driverConf struct {
	Influx          Influx `json:"influx,omitempty"`
	SinkKafkaStream string `json:"sink_kafka_stream,omitempty"`
	KubeNamespace   string `json:"kube_namespace,omitempty"`
	PrometheusURL   string `json:"prometheus_url,omitempty"`
	FlinkName       string `json:"flink_name,omitempty"`
}

type Output struct {
	KubeCluster     kubernetes.Output `json:"kube_cluster,omitempty"`
	KubeNamespace   string            `json:"kube_namespace,omitempty"`
	Influx          Influx            `json:"influx,omitempty"`
	SinkKafkaStream string            `json:"sink_kafka_stream,omitempty"`
	PrometheusURL   string            `json:"prometheus_url,omitempty"`
	FlinkName       string            `json:"flink_name,omitempty"`
	ExtraStreams    []string          `json:"extra_streams,omitempty"`
}

func readOutputData(exr module.ExpandedResource) (*Output, error) {
	var curOut Output
	if len(exr.Resource.State.Output) == 0 {
		return &curOut, nil
	}
	if err := json.Unmarshal(exr.Resource.State.Output, &curOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted output").WithCausef(err.Error())
	}
	return &curOut, nil
}
