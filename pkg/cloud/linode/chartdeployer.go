package linode

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesbase "github.com/gardener/gardener/pkg/client/kubernetes/base"
	"github.com/golang/glog"
	"golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// A ChartDeployer can be used to deploy Helm charts to a target Kubernetes cluster
type ChartDeployer struct {
	client   kubernetes.Client
	renderer chartrenderer.ChartRenderer
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
		client:   client,
		renderer: renderer,
	}, nil
}

// DeployChart deploys a Helm chart to the configured cluster to the target namespace.
// It does not use Tiller; everything is templated locally and applied to the cluster.
// A map of template values can be provided.
func (cd *ChartDeployer) DeployChart(chartPath, namespace string, values map[string]interface{}) error {
	// use the chartPath as the releaseName, because we're in a cluster-specific namespace
	renderedChart, err := cd.renderer.Render(chartPath, chartPath, namespace, values)
	if err != nil {
		return err
	}
	glog.V(3).Infof("[%s] Deploying chart from path: %s", namespace, chartPath)
	glog.V(4).Infof("[%s] Deploying the following manifest:", namespace)
	glog.V(4).Infof("%v", renderedChart.Files)
	return cd.client.Apply(renderedChart.Manifest())
}

/*
 * tempfile creates a temporary file, writes data to it, and returns the file name
 */
func tempfile(prefix string, data []byte) (string, error) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		return "", err
	}

	if _, err := file.Write(data); err != nil {
		return "", err
	}

	if err := file.Close(); err != nil {
		return "", err
	}

	return file.Name(), nil
}

/*
 * tempKubeconfig creates a temporary file containing admin config for a LKE
 * cluster specified in the arguments and returns the name of this new file.
 */
func tempKubeconfig(cpcClient client.Client, clusterNamespace string) (string, error) {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: clusterNamespace, Name: "admin-kubeconfig"}

	err := cpcClient.Get(context.Background(), namespacedName, secret)
	if err != nil {
		return "", err
	}

	if len(secret.Data["admin.conf"]) == 0 {
		return "", fmt.Errorf("[%s] admin-kubeconfig secret: admin.conf: empty", clusterNamespace)
	}

	return tempfile("lkeconfig", secret.Data["admin.conf"])
}

/*
 * lkeClient returns an LKE client based on its arguments. The credentials for
 * the LKE cluster are taken from the CPC using cpcClient
 */
func lkeChartClient(cpcClient client.Client, clusterNamespace string) (*kubernetesbase.Client, error) {
	kubeconfig, err := tempKubeconfig(cpcClient, clusterNamespace)
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return kubernetesbase.NewForConfig(config)
}

/*
 * The newChartDeployerLKE function creates a new ChartDeployer which points to
 * a LKE cluster specified in the arguments. The cpcClient argument is used to
 * grab LKE credentials from CPC (kept as a secret).
 */
func newChartDeployerLKE(cpcClient client.Client, clusterNamespace string) (*ChartDeployer, error) {

	client, err := lkeChartClient(cpcClient, clusterNamespace)
	if err != nil {
		return nil, err
	}
	renderer, err := chartrenderer.New(client)
	if err != nil {
		return nil, err
	}
	return &ChartDeployer{
		client:   client,
		renderer: renderer,
	}, nil
}
