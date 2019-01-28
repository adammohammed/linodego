package linode

import (
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/base"
	"k8s.io/client-go/rest"
)

type ChartDeployer struct {
	cpcClient kubernetes.Client
	renderer  chartrenderer.ChartRenderer
}

func newChartDeployer(config *rest.Config) (*ChartDeployer, error) {
	client, err := kubernetesbase.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	renderer, err := chartrenderer.New(client)
	if err != nil {
		return nil, err
	}
	return &ChartDeployer{
		cpcClient: client,
		renderer:  renderer,
	}, nil
}

func (cd *ChartDeployer) DeployChart(chartPath, namespace string, values map[string]interface{}) error {
	return nil
}
