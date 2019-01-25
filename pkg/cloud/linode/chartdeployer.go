package linode

import (
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChartDeployer struct {
	cpcClient kubernetes.Interface
	renderer  chartrenderer.ChartRenderer
}

func newChartDeployer(config *rest.Config) (*ChartDeployer, error) {
	client, err := kubernetes.NewForConfig(config, client.Options{})
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
