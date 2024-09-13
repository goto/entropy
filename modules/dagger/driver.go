package dagger

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
)

const (
	stepReleaseCreate = "release_create"
)

const (
	chartRepo = "https://goto.github.io/charts/"
	chartName = "dagger-deployment-chart"
	imageRepo = "gotocompany/dagger"
)

const (
	labelsConfKey = "extra_labels"

	labelDeployment   = "deployment"
	labelOrchestrator = "orchestrator"
	labelURN          = "urn"
	labelName         = "name"
	labelNamespace    = "namespace"

	orchestratorLabelValue = "entropy"
)

const defaultKey = "default"

var defaultDriverConf = driverConf{
	Namespace: map[string]string{
		defaultKey: "dagger",
	},
	ChartValues: ChartValues{
		ChartVersion: "0.1.0",
	},
}

type daggerDriver struct {
	timeNow    func() time.Time
	conf       driverConf
	kubeDeploy kubeDeployFn
	kubeGetPod kubeGetPodFn
}

type (
	kubeDeployFn func(ctx context.Context, isCreate bool, conf kube.Config, hc helm.ReleaseConfig) error
	kubeGetPodFn func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error)
)

type driverConf struct {
	// Labels to be injected to the chart during deployment. Values can be Go templates.
	Labels map[string]string `json:"labels,omitempty"`

	// Namespace is the kubernetes namespace where firehoses will be deployed.
	Namespace map[string]string `json:"namespace" validate:"required"`

	// ChartValues is the chart and image version information.
	ChartValues ChartValues `json:"chart_values" validate:"required"`

	EnvVariables map[string]string `json:"env_variables,omitempty"`

	Resources Resources `json:"resources" validate:"required"`

	JarURI string `json:"jar_uri" validate:"required"`

	// timeout value for a kube deployment run
	KubeDeployTimeout int `json:"kube_deploy_timeout_seconds"`
}

type Output struct {
	State          string     `json:"state,omitempty"`
	JMDeployStatus string     `json:"jm_deploy_status,omitempty"`
	JobStatus      string     `json:"job_status,omitempty"`
	Namespace      string     `json:"namespace,omitempty"`
	ReleaseName    string     `json:"release_name,omitempty"`
	Pods           []kube.Pod `json:"pods,omitempty"`
}

type transientData struct {
	PendingSteps []string `json:"pending_steps"`
}

func mergeChartValues(cur, newVal *ChartValues) (*ChartValues, error) {
	if newVal == nil {
		return cur, nil
	}

	merged := ChartValues{
		ChartVersion: newVal.ChartVersion,
	}

	return &merged, nil
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

func readTransientData(exr module.ExpandedResource) (*transientData, error) {
	if len(exr.Resource.State.ModuleData) == 0 {
		return &transientData{}, nil
	}

	var modData transientData
	if err := json.Unmarshal(exr.Resource.State.ModuleData, &modData); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted transient data").WithCausef(err.Error())
	}
	return &modData, nil
}

func (dd *daggerDriver) getHelmRelease(res resource.Resource, conf Config,
	kubeOut kubernetes.Output,
) (*helm.ReleaseConfig, error) {

	entropyLabels := map[string]string{
		labelDeployment:   conf.DeploymentID,
		labelOrchestrator: orchestratorLabelValue,
	}

	otherLabels := map[string]string{
		labelURN:       res.URN,
		labelName:      res.Name,
		labelNamespace: conf.Namespace,
	}

	deploymentLabels, err := renderTpl(dd.conf.Labels, modules.CloneAndMergeMaps(res.Labels, modules.CloneAndMergeMaps(entropyLabels, otherLabels)))
	if err != nil {
		return nil, err
	}

	rc := helm.DefaultReleaseConfig()
	rc.Timeout = dd.conf.KubeDeployTimeout
	rc.Name = conf.DeploymentID
	rc.Repository = chartRepo
	rc.Chart = chartName
	rc.Namespace = conf.Namespace
	rc.ForceUpdate = true
	rc.Version = conf.ChartValues.ChartVersion

	imageRepository := dd.conf.ChartValues.ImageRepository
	if conf.ChartValues.ImageRepository != "" {
		imageRepository = conf.ChartValues.ImageRepository
	}

	_, err = json.Marshal(conf.EnvVariables)
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("failed to marshal env variables").WithCausef(err.Error())
	}

	//encodedEnvVars := base64.StdEncoding.EncodeToString(envVarsJSON)
	//programArgs := encodedEnvVars

	rc.Values = map[string]any{
		labelsConfKey:   modules.CloneAndMergeMaps(deploymentLabels, entropyLabels),
		"image":         imageRepository,
		"deployment_id": conf.DeploymentID,
		"configuration": map[string]any{
			"FLINK_PARALLELISM": conf.Replicas,
		},
		"projectID":      res.Project,
		"name":           res.Name,
		"team":           res.Labels["team"],                      //TODO: improve handling this case
		"flink_name":     "g-pilotdata-gl-playground-operator-v2", //TODO: use this from flink resource
		"prometheus_url": conf.PrometheusURL,
		"resources": map[string]any{
			"jobmanager": map[string]any{
				"cpu":    conf.Resources.JobManager.CPU,
				"memory": conf.Resources.JobManager.Memory,
			},
			"taskmanager": map[string]any{
				"cpu":    conf.Resources.TaskManager.CPU,
				"memory": conf.Resources.JobManager.Memory,
			},
		},
		"jarURI": conf.JarURI,
		//TODO: build programArgs from EnvVariables
		"programArgs": []string{"--encodedArgs", "WyItLVNPVVJDRV9LQUZLQV9DT05TVU1FX0xBUkdFX01FU1NBR0VfRU5BQkxFIixmYWxzZSwiLS1FTkFCTEVfU1RFTkNJTF9VUkwiLHRydWUsIi0tRkxJTktfSk9CX0lEIiwiZy1waWxvdGRhdGEtZ2wtdGVzdC1kb2NrZXItZXJyb3ItZGFnZ2VyIiwiLS1TSU5LX0lORkxVWF9CQVRDSF9TSVpFIiwxMDAsIi0tU0lOS19JTkZMVVhfREJfTkFNRSIsIkRBR0dFUlNfUExBWUdST1VORCIsIi0tU0lOS19JTkZMVVhfRkxVU0hfRFVSQVRJT05fTVMiLDEwMDAsIi0tU0lOS19JTkZMVVhfTUVBU1VSRU1FTlRfTkFNRSIsInRlc3QtZG9ja2VyLWVycm9yIiwiLS1TSU5LX0lORkxVWF9QQVNTV09SRCIsInJvb3QiLCItLVNJTktfSU5GTFVYX1JFVEVOVElPTl9QT0xJQ1kiLCJhdXRvZ2VuIiwiLS1TSU5LX0lORkxVWF9VUkwiLCJodHRwOi8vMTAuODQuNTIuODk6ODA4NiIsIi0tU0lOS19JTkZMVVhfVVNFUk5BTUUiLCJyb290IiwiLS1QUk9DRVNTT1JfTE9OR0JPV19HQ1BfSU5TVEFOQ0VfSUQiLCJ0ZXN0IiwiLS1QUk9DRVNTT1JfTE9OR0JPV19HQ1BfUFJPSkVDVF9JRCIsInRlc3QiLCItLVNJTktfS0FGS0FfQlJPS0VSUyIsIiIsIi0tU0lOS19LQUZLQV9UT1BJQyIsIiIsIi0tT1VUUFVUX1BST1RPX0NMQVNTX1BSRUZJWCIsIiIsIi0tU0lOS19LQUZLQV9QUk9UT19LRVkiLCIiLCItLVNJTktfS0FGS0FfUFJPVE9fTUVTU0FHRSIsIiIsIi0tU0lOS19LQUZLQV9TVFJFQU0iLCIiLCItLUZMSU5LX1BBUkFMTEVMSVNNIiwxLCItLVBPUlRBTF9WRVJTSU9OIiwiMiIsIi0tUFJPQ0VTU09SX1BSRVBST0NFU1NPUl9DT05GSUciLCIiLCItLVBST0NFU1NPUl9QUkVQUk9DRVNTT1JfRU5BQkxFIixmYWxzZSwiLS1QUk9DRVNTT1JfUE9TVFBST0NFU1NPUl9DT05GSUciLCIiLCItLVBST0NFU1NPUl9QT1NUUFJPQ0VTU09SX0VOQUJMRSIsZmFsc2UsIi0tU0lOS19LQUZLQV9QUk9EVUNFX0xBUkdFX01FU1NBR0VfRU5BQkxFIixmYWxzZSwiLS1SRURJU19TRVJWRVIiLCIiLCItLUZMSU5LX1JPV1RJTUVfQVRUUklCVVRFX05BTUUiLCJyb3d0aW1lIiwiLS1TSU5LX1RZUEUiLCJpbmZsdXgiLCItLUZMSU5LX1NRTF9RVUVSWSIsIlNFTEVDVCAqIEZST00gZGF0YV9zdHJlYW1zXzAiLCItLVNDSEVNQV9SRUdJU1RSWV9TVEVOQ0lMX0VOQUJMRSIsdHJ1ZSwiLS1TQ0hFTUFfUkVHSVNUUllfU1RFTkNJTF9VUkxTIiwiaHR0cDovL2ctZ29kYXRhLXN5c3RlbXMtc3RlbmNpbC12MWJldGExLWluZ3Jlc3MuZ29sYWJzLmlvL3YxYmV0YTEvbmFtZXNwYWNlcy9nb2play9zY2hlbWFzL2VzYi1sb2ctZW50aXRpZXMvdmVyc2lvbnMvOCIsIi0tU1RSRUFNUyIsIlt7XCJJTlBVVF9TQ0hFTUFfRVZFTlRfVElNRVNUQU1QX0ZJRUxEX0lOREVYXCI6XCI1XCIsXCJJTlBVVF9TQ0hFTUFfUFJPVE9fQ0xBU1NcIjpcImdvamVrLmVzYi5ib29raW5nLkJvb2tpbmdMb2dNZXNzYWdlXCIsXCJJTlBVVF9TQ0hFTUFfVEFCTEVcIjpcImRhdGFfc3RyZWFtc18wXCIsXCJTT1VSQ0VfREVUQUlMU1wiOlt7XCJTT1VSQ0VfTkFNRVwiOlwiS0FGS0FfQ09OU1VNRVJcIixcIlNPVVJDRV9UWVBFXCI6XCJVTkJPVU5ERURcIn1dLFwiU09VUkNFX0tBRktBX0NPTlNVTUVSX0NPTkZJR19BVVRPX0NPTU1JVF9FTkFCTEVcIjpcImZhbHNlXCIsXCJTT1VSQ0VfS0FGS0FfQ09OU1VNRVJfQ09ORklHX0FVVE9fT0ZGU0VUX1JFU0VUXCI6XCJsYXRlc3RcIixcIlNPVVJDRV9LQUZLQV9DT05TVU1FUl9DT05GSUdfQk9PVFNUUkFQX1NFUlZFUlNcIjpcImctcGlsb3RkYXRhLWdsLW1haW5zdHJlYW0ta2Fma2EuZ29sYWJzLmlvOjY2NjhcIixcIlNPVVJDRV9LQUZLQV9DT05TVU1FUl9DT05GSUdfR1JPVVBfSURcIjpcImctcGlsb3RkYXRhLWdsLWRhZ2dlci10ZXN0LTAxLWRhZ2dlci0wMDAxXCIsXCJTT1VSQ0VfS0FGS0FfTkFNRVwiOlwiZy1waWxvdGRhdGEtZ2wtbWFpbnN0cmVhbVwiLFwiU09VUkNFX0tBRktBX1RPUElDX05BTUVTXCI6XCJhZ2dyZWdhdGVkZGVtYW5kLXByaWNpbmd0ZXN0XCIsXCJTT1VSQ0VfUEFSUVVFVF9GSUxFX0RBVEVfUkFOR0VcIjpudWxsLFwiU09VUkNFX1BBUlFVRVRfRklMRV9QQVRIU1wiOm51bGx9XSIsIi0tRkxJTktfV0FURVJNQVJLX0RFTEFZX01TIiwxMDAwLCItLUZMSU5LX1dBVEVSTUFSS19JTlRFUlZBTF9NUyIsNjAwMDAsIi0tVURGX0RBUlRfR0NTX0JVQ0tFVF9JRCIsInAtZ29kYXRhLWRhZ2dlcnMtZGFydHMtc3RvcmFnZSIsIi0tVURGX0RBUlRfR0NTX1BST0pFQ1RfSUQiLCJnb2RhdGEtcHJvZHVjdGlvbiIsIi0tRlVOQ1RJT05fRkFDVE9SWV9DTEFTU0VTIiwiY29tLmdvdG9jb21wYW55LmRhZ2dlci5mdW5jdGlvbnMudWRmcy5mYWN0b3JpZXMuRnVuY3Rpb25GYWN0b3J5LGNvbS5nb2play5kZS5mbHVkLnVkZnMuZmFjdG9yaWVzLkZ1bmN0aW9uRmFjdG9yeSIsIi0tUFlUSE9OX1VERl9FTkFCTEUiLGZhbHNlLCItLVBZVEhPTl9VREZfQ09ORklHIiwie1wiUFlUSE9OX0ZJTEVTXCI6IFwiZ3M6Ly9nb2RhdGEtZGFnZ2VyL3B5dGhvbi9tYXN0ZXIvbGF0ZXN0L3B5dGhvbl91ZGZzLnppcFwiLFwiUFlUSE9OX1JFUVVJUkVNRU5UU1wiOiBcImdzOi8vZ29kYXRhLWRhZ2dlci9weXRob24vbWFzdGVyL2xhdGVzdC9yZXF1aXJlbWVudHMudHh0XCIsXCJQWVRIT05fQVJDSElWRVNcIjogXCJnczovL2dvZGF0YS1kYWdnZXIvcHl0aG9uL21hc3Rlci9sYXRlc3QvZGF0YS56aXAjZGF0YVwiLFwiUFlUSE9OX0ZOX0VYRUNVVElPTl9BUlJPV19CQVRDSF9TSVpFXCI6IFwiMTAwMDBcIixcIlBZVEhPTl9GTl9FWEVDVVRJT05fQlVORExFX1NJWkVcIjogXCIxMDAwMDBcIixcIlBZVEhPTl9GTl9FWEVDVVRJT05fQlVORExFX1RJTUVcIjogXCIxMDAwXCJ9IiwiLS1TSU5LX0JJR1FVRVJZX0dPT0dMRV9DTE9VRF9QUk9KRUNUX0lEIiwiIiwiLS1TSU5LX0JJR1FVRVJZX1RBQkxFX05BTUUiLCIiLCItLVNJTktfQklHUVVFUllfREFUQVNFVF9MQUJFTFMiLCIiLCItLVNJTktfQklHUVVFUllfVEFCTEVfTEFCRUxTIiwiIiwiLS1TSU5LX0JJR1FVRVJZX0RBVEFTRVRfTkFNRSIsIiIsIi0tU0lOS19CSUdRVUVSWV9DUkVERU5USUFMX1BBVEgiLCIvdmFyL3NlY3JldHMvZ29vZ2xlL2djcF9rZXkuanNvbiIsIi0tU0lOS19CSUdRVUVSWV9UQUJMRV9QQVJUSVRJT05JTkdfRU5BQkxFIixmYWxzZSwiLS1TSU5LX0JJR1FVRVJZX1RBQkxFX1BBUlRJVElPTl9LRVkiLCIiLCItLVNJTktfQklHUVVFUllfUk9XX0lOU0VSVF9JRF9FTkFCTEUiLGZhbHNlLCItLVNJTktfQklHUVVFUllfQ0xJRU5UX1JFQURfVElNRU9VVF9NUyIsLTEsIi0tU0lOS19CSUdRVUVSWV9DTElFTlRfQ09OTkVDVF9USU1FT1VUX01TIiwtMSwiLS1TSU5LX0JJR1FVRVJZX1RBQkxFX1BBUlRJVElPTl9FWFBJUllfTVMiLC0xLCItLVNJTktfQklHUVVFUllfREFUQVNFVF9MT0NBVElPTiIsImFzaWEtc291dGhlYXN0MSIsIi0tU0lOS19CSUdRVUVSWV9NRVRBREFUQV9OQU1FU1BBQ0UiLCIiLCItLVNJTktfQklHUVVFUllfQUREX01FVEFEQVRBX0VOQUJMRUQiLGZhbHNlLCItLVNJTktfQklHUVVFUllfTUVUQURBVEFfQ09MVU1OU19UWVBFUyIsIiIsIi0tU0lOS19NRVRSSUNTX0FQUExJQ0FUSU9OX1BSRUZJWCIsImRhZ2dlcl8iLCItLVNJTktfQklHUVVFUllfQkFUQ0hfU0laRSIsIiIsIi0tU0lOS19DT05ORUNUT1JfU0NIRU1BX1BST1RPX01FU1NBR0VfQ0xBU1MiLCIiLCItLVNJTktfQ09OTkVDVE9SX1NDSEVNQV9QUk9UT19LRVlfQ0xBU1MiLCIiLCItLVNJTktfQ09OTkVDVE9SX1NDSEVNQV9EQVRBX1RZUEUiLCJQUk9UT0JVRiIsIi0tU0lOS19DT05ORUNUT1JfU0NIRU1BX01FU1NBR0VfTU9ERSIsIkxPR19NRVNTQUdFIiwiLS1TSU5LX0NPTk5FQ1RPUl9TQ0hFTUFfUFJPVE9fQUxMT1dfVU5LTk9XTl9GSUVMRFNfRU5BQkxFIixmYWxzZSwiLS1TSU5LX0VSUk9SX1RZUEVTX0ZPUl9GQUlMVVJFIiwiIiwiLS1TSU5LX0JJR1FVRVJZX1RBQkxFX0NMVVNURVJJTkdfRU5BQkxFIixmYWxzZSwiLS1TSU5LX0JJR1FVRVJZX1RBQkxFX0NMVVNURVJJTkdfS0VZUyIsIiIsIi0tU0NIRU1BX1JFR0lTVFJZX1NURU5DSUxfQ0FDSEVfQVVUT19SRUZSRVNIIixmYWxzZSwiLS1TQ0hFTUFfUkVHSVNUUllfU1RFTkNJTF9SRUZSRVNIX1NUUkFURUdZIiwiTE9OR19QT0xMSU5HIiwiLS1TQ0hFTUFfUkVHSVNUUllfU1RFTkNJTF9DQUNIRV9UVExfTVMiLCI5MDAwMDAiLCItLVNJTktfS0FGS0FfTElOR0VSX01TIiwiMCIsIi0tU0NIRU1BX1JFR0lTVFJZX1NURU5DSUxfRkVUQ0hfUkVUUklFUyIsIjQiLCItLVNDSEVNQV9SRUdJU1RSWV9TVEVOQ0lMX0ZFVENIX0JBQ0tPRkZfTUlOX01TIiwiNjAwMDAiLCItLVNDSEVNQV9SRUdJU1RSWV9TVEVOQ0lMX0ZFVENIX1RJTUVPVVRfTVMiLCIxMDAwIl0="},
		"state":       "running",
		"namespace":   conf.Namespace,
	}

	return rc, nil
}

// TODO: move this to pkg
func renderTpl(labelsTpl map[string]string, labelsValues map[string]string) (map[string]string, error) {
	const useZeroValueForMissingKey = "missingkey=zero"

	finalLabels := map[string]string{}
	for k, v := range labelsTpl {
		var buf bytes.Buffer
		t, err := template.New("").Option(useZeroValueForMissingKey).Parse(v)
		if err != nil {
			return nil, errors.ErrInvalid.
				WithMsgf("label template for '%s' is invalid", k).WithCausef(err.Error())
		} else if err := t.Execute(&buf, labelsValues); err != nil {
			return nil, errors.ErrInvalid.
				WithMsgf("failed to render label template").WithCausef(err.Error())
		}

		// allow empty values
		//		labelVal := strings.TrimSpace(buf.String())
		//		if labelVal == "" {
		//			continue
		//		}

		finalLabels[k] = buf.String()
	}
	return finalLabels, nil
}
