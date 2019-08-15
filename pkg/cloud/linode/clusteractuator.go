/*
Copyright 2018-2019 Linode, LLC.
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package linode

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"encoding/json"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const chartPath = "charts"

type LinodeClusterClient struct {
	client        client.Client
	chartDeployer *ChartDeployer
}

type ClusterActuatorParams struct {
}

func NewClusterActuator(m manager.Manager, params ClusterActuatorParams) (*LinodeClusterClient, error) {
	chartDeployer, err := newChartDeployer(m.GetConfig())
	if err != nil {
		return nil, err
	}

	return &LinodeClusterClient{
		client:        m.GetClient(),
		chartDeployer: chartDeployer,
	}, nil
}

type Version struct {
	s string
}

func (v Version) String() string {
	return v.s
}

func (v Version) K8S() string {
	if v.s == "bleeding" {
		return "v1.13"
	}
	return strings.Split(v.s, "-")[0]
}

func (v Version) Caplke() string {
	if v.s == "bleeding" {
		return v.s
	}
	return strings.Split(v.s, "-")[1]
}

func getVersion(cluster *clusterv1.Cluster) (Version, error) {
	version_key := "lke.linode.com/caplke-version"
	version_str := cluster.ObjectMeta.Annotations[version_key]
	if len(version_str) == 0 {
		return Version{}, fmt.Errorf("[] empty annotation: %s", version_key)
	}

	// XXX: if version is given without the caplke extention we should update it with (the latest) one

	return Version{s: version_str}, nil
}

type SecretDesc struct {
	Name      string
	Type      string
	Finalizer string
}

type Resource struct {
	Kind string
	Name string
}

type ChartDescription struct {
	Resources       []Resource
	SecrtesRequired []SecretDesc

	// the following vars are private and filled by code
	path string
}

type ChartSet struct {
	// the following values are unmarshalled from the config
	CpcCharts             []string
	LkeCharts             []string
	CpcSecrets            []SecretDesc
	LkeSecrets            []SecretDesc
	SecrtesRequiredFormat map[string][]string

	// the following values are unmarshalled indirectly from chart configs
	chartDescriptions map[string]ChartDescription

	// the following vars are private and filled by code
	path    string
	version Version // needed? I hope no
}

/*
 * Validate that LKE services are deployed and running with expected
 * configuration. If they are not, deploy or modify them.
 */
func (lcc *LinodeClusterClient) reconcile(cluster *clusterv1.Cluster) error {
	version, err := getVersion(cluster)
	if err != nil {
		return err
	}

	chartSet, err := getChartSet(version)
	if err != nil {
		return err
	}

	ip, err := lcc.getAPIServerIP(cluster, version)
	if err != nil {
		return err
	}

	secretsCache, err := lcc.reconcileSecrets(cluster, version, chartSet)
	if err != nil {
		return err
	}

	values := getCpcChartValues(secretsCache, cluster.Name, ip)
	if err := chartSet.reconcileCPC(lcc, cluster, secretsCache, values); err != nil {
		return err
	}

	lkeClient, err := lkeClientNew(lcc.client, cluster.Name)
	if err != nil {
		return err
	}

	chartDeployerLKE, err := newChartDeployerLKE(lcc.client, cluster.Name)
	if err != nil {
		return err
	}

	if err := chartSet.reconcileLKE(lkeClient, chartDeployerLKE, secretsCache); err != nil {
		return err
	}

	if err := lcc.reconcileAddonsAndConfigmaps(cluster, version, ip, lkeClient); err != nil {
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) Reconcile(cluster *clusterv1.Cluster) error {

	glog.Infof("[%v] reconciling", cluster.Name)
	start := time.Now()

	if err := lcc.reconcile(cluster); err != nil {
		glog.Infof("[%v] reconcilation error [time spent %s]: %v", cluster.Name, time.Since(start), err)
		return err
	}

	glog.Infof("[%v] reconcilation complete [time spent %s]", cluster.Name, time.Since(start))
	return nil
}

/////////////////////////////////////////////////////////////////////

func (chartSet *ChartSet) checkResources(client client.Client, namespace string, chartDesc ChartDescription) (bool, error) {

	// always apply bleeding stuff
	if chartSet.version.Caplke() == "bleeding" {
		return false, nil
	}

	// if any resource is outdated or can't be checked, then re-apply the chart
	for _, r := range chartDesc.Resources {
		if v, err := getResourceVersion(client, namespace, &r); err != nil {
			return false, err
		} else if v != chartSet.version.String() {
			return false, nil
		}
	}

	// all resources are up-to-date
	return true, nil
}

func (chartSet *ChartSet) checkChartSecretRequirements(chart string, secretsCache SecretsCache) error {
	if desc, ok := chartSet.chartDescriptions[chart]; !ok {
		return fmt.Errorf("chart %s wasn't found", chart)
	} else /* ok */ {
		for _, secretDesc := range desc.SecrtesRequired {
			if _, ok := secretsCache[secretDesc.Name]; !ok {
				return fmt.Errorf("chart %s requires %s secret which doesn't exist", chart, secretDesc.Name)
			}
		}
	}
	return nil
}

func (chartSet *ChartSet) Checks(version Version, secretsCache SecretsCache) error {
	return nil
}

func (chartSet *ChartSet) validSecretFormat(secret *corev1.Secret) error {
	name := secret.ObjectMeta.Name
	if format, ok := chartSet.SecrtesRequiredFormat[name]; ok {
		for _, item := range format {
			if _, ok := secret.Data[item]; !ok {
				return fmt.Errorf("secret %s should contain the %s data item", name, item)
			}
		}
	}
	return nil
}

func (chartSet *ChartSet) copyCPCSecret(client client.Client, ns string, secretDesc SecretDesc) (map[string][]byte, error) {
	name := secretDesc.Name

	// if this secret exists, then check that it has the right format
	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, secret); err == nil {
		if err := chartSet.validSecretFormat(secret); err != nil {
			// try to delete invalid secret
			if err := client.Delete(context.Background(), secret); err != nil {
				return nil, err
			}
		} else {
			// exists and valid
			return secret.Data, nil
		}
	}

	// get the parent secret
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "kube-system", Name: name}, secret); err != nil {
		return nil, fmt.Errorf("can't get parent secret %s: %v", name, err)
	} else if err := chartSet.validSecretFormat(secret); err != nil {
		return nil, fmt.Errorf("invalid parent secret %s: %v", name, err)
	}

	switch secretDesc.Type {
	case "opaque":
		return secret.Data, createOpaqueSecret(client, ns, name, secret.Data, false, secretDesc.Finalizer)
	case "kubernetes.io/dockerconfigjson":
		return secret.Data, createDockerSecret(client, ns, name, secret.Data, false, secretDesc.Finalizer)
	default:
		return nil, fmt.Errorf("unsupported secret type: %v", secretDesc.Type)
	}
}

func (chartSet *ChartSet) copyLkeSecret(client client.Client, secretDesc SecretDesc, secretsCache SecretsCache) error {
	name := secretDesc.Name

	data := map[string][]byte{}
	ok := false

	if data, ok = secretsCache[name]; !ok {
		return fmt.Errorf("can't find secret %s in cache", name)
	}

	// if this secret exists, then check that it has the right format
	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "kube-system", Name: name}, secret); err == nil {
		if err := chartSet.validSecretFormat(secret); err != nil {
			// try to delete invalid secret
			if err := client.Delete(context.Background(), secret); err != nil {
				return err
			}
		} else {
			// exists and valid
			return nil
		}
	}

	switch secretDesc.Type {
	case "opaque":
		return createOpaqueSecret(client, "kube-system", name, data, false, "")
	case "kubernetes.io/dockerconfigjson":
		return createDockerSecret(client, "kube-system", name, data, false, "")
	default:
		return fmt.Errorf("unsupported secret type: %v", secretDesc.Type)
	}
}

func (chartSet *ChartSet) reconcileCPCChart(lcc *LinodeClusterClient, cluster *clusterv1.Cluster, chart string, values map[string]interface{}) error {
	if chartDesc, ok := chartSet.chartDescriptions[chart]; !ok {
		return fmt.Errorf("chart %s isn't listed in resources", chart)
	} else {
		if uptodate, err := chartSet.checkResources(lcc.client, clusterNamespace(cluster.Name), chartDesc); err != nil {
			return err
		} else if !uptodate {
			if err := lcc.chartDeployer.DeployChart(chartDesc.path, cluster.Name, values); err != nil {
				return err
			}
		}
	}
	return nil
}

func (chartSet *ChartSet) reconcileCPC(lcc *LinodeClusterClient, cluster *clusterv1.Cluster, secretsCache SecretsCache, values map[string]interface{}) error {

	ns := cluster.GetNamespace()

	for _, secretDesc := range chartSet.CpcSecrets {
		if secretData, err := chartSet.copyCPCSecret(lcc.client, ns, secretDesc); err != nil {
			return fmt.Errorf("Error copying the %v secret to the LKE namespace: %v", secretDesc, err)
		} else {
			secretsCache[secretDesc.Name] = secretData
		}
	}

	for _, chart := range chartSet.CpcCharts {
		if err := chartSet.checkChartSecretRequirements(chart, secretsCache); err != nil {
			return err
		}
	}

	for _, chart := range chartSet.CpcCharts {
		if err := chartSet.reconcileCPCChart(lcc, cluster, chart, values); err != nil {
			return err
		}
	}

	return nil
}

func (chartSet *ChartSet) reconcileLKEChart(client client.Client, chartDeployer *ChartDeployer, chart string) error {
	if chartDesc, ok := chartSet.chartDescriptions[chart]; !ok {
		return fmt.Errorf("chart %s isn't listed in resources", chart)
	} else {
		if uptodate, err := chartSet.checkResources(client, "kube-system", chartDesc); err != nil {
			return err
		} else if !uptodate {
			if err := chartDeployer.DeployChart(chartDesc.path, "kube-system", map[string]interface{}{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (chartSet *ChartSet) reconcileLKE(client client.Client, chartDeployer *ChartDeployer, secretsCache SecretsCache) error {
	for _, secretDesc := range chartSet.LkeSecrets {
		if err := chartSet.copyLkeSecret(client, secretDesc, secretsCache); err != nil {
			return fmt.Errorf("Error copying the %s secret to the LKE namespace: %v", secretDesc.Name, err)
		}
	}

	// XXX: do we need to check that secret requirements are satisfied in LKE cluster?

	for _, chart := range chartSet.LkeCharts {
		if err := chartSet.reconcileLKEChart(client, chartDeployer, chart); err != nil {
			return err
		}
	}

	return nil
}

func (chartSet *ChartSet) readChartDescription(chart string) (*ChartDescription, error) {

	var desc ChartDescription
	desc.path = fmt.Sprintf("%s/%s", chartSet.path, chart)

	f, err := os.Open(desc.path + "/" + "config.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &desc); err != nil {
		return nil, err
	}

	if len(desc.Resources) == 0 {
		return nil, fmt.Errorf("chart %s: no resources found", chart)
	}

	return &desc, nil
}

func (chartSet *ChartSet) readChartDescriptions() error {
	for _, chart := range append(chartSet.CpcCharts, chartSet.LkeCharts...) {
		if desc, err := chartSet.readChartDescription(chart); err != nil {
			return err
		} else {
			chartSet.chartDescriptions[chart] = *desc
		}
	}
	return nil
}

func getChartSet(version Version) (*ChartSet, error) {
	// cache non-bleeding

	var chartSet ChartSet
	chartSet.path = fmt.Sprintf("%s/%s", chartPath, version)
	chartSet.version = version
	chartSet.chartDescriptions = make(map[string]ChartDescription)

	f, err := os.Open(chartSet.path + "/" + "config.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &chartSet); err != nil {
		return nil, err
	}

	if err := chartSet.readChartDescriptions(); err != nil {
		return nil, err
	}

	return &chartSet, nil
}

func getCpcChartValues(secretsCache SecretsCache, clusterName, ip string) map[string]interface{} {

	// all these values are to be removed, place all info in secrets and export as environment variables, if needed
	linodeSecret := secretsCache["linode"]
	return map[string]interface{}{
		// StorePrefix example: us-east/cpc1190/12ahd312/lke123123 // XXX should go away
		"StorePrefix": fmt.Sprintf("%s/cpc%s/%s/%s",
			linodeSecret["region"],
			linodeSecret["id"],
			linodeSecret["timestamp"],
			clusterName),
		// XXX only three files use ClusterName, remove this dependency
		//charts/bleeding/apiserver/templates/apiserver.yaml
		//charts/bleeding/controller-manager/templates/controller-manager.yaml
		//charts/bleeding/scheduler/templates/scheduler.yaml
		"ClusterName": clusterName,
		// XXX: only charts/bleeding/apiserver/templates/apiserver.yaml uses AdvertiseAddress
		"AdvertiseAddress": ip,
	}
}

func (lcc *LinodeClusterClient) reconcileAPIServerService(cluster *clusterv1.Cluster, version Version) error {
	apiService := &corev1.Service{}
	apiService.ObjectMeta = metav1.ObjectMeta{
		Namespace: cluster.GetNamespace(),
		Name:      "kube-apiserver",
		Labels: map[string]string{
			"run": "kube-apiserver",
		},
		Annotations: map[string]string{
			"lke.linode.com/caplke-version":                           version.String(),
			"service.beta.kubernetes.io/linode-loadbalancer-protocol": "tcp",
		},
	}
	apiService.Spec = corev1.ServiceSpec{
		Type:     corev1.ServiceTypeLoadBalancer,
		Selector: map[string]string{"run": "kube-apiserver"},
		Ports: []corev1.ServicePort{{
			Name:       "https",
			Protocol:   corev1.ProtocolTCP,
			Port:       6443,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 6443},
		}},
	}
	return lcc.client.Create(context.Background(), apiService)
}

func (lcc *LinodeClusterClient) getAPIServerIP(cluster *clusterv1.Cluster, version Version) (string, error) {

	/* If service doesn't exist then we will try to create it */
	apiService := &corev1.Service{}
	nn := types.NamespacedName{Namespace: cluster.GetNamespace(), Name: "kube-apiserver"}
	if err := lcc.client.Get(context.Background(), nn, apiService); err != nil {
		if err := lcc.reconcileAPIServerService(cluster, version); err != nil {
			return "", err
		}
	}
	glog.Infof("Found service for kube-apiserver for cluster %v: %v", cluster.Name, apiService.Name)
	if len(apiService.Status.LoadBalancer.Ingress) < 1 {
		return "", fmt.Errorf("No ExternalIPs yet for kube-apiserver for cluster %v", cluster.Name)
	}
	ip := apiService.Status.LoadBalancer.Ingress[0].IP

	// Write that NodeBalancer address as the cluster API endpoint
	glog.Infof("External IP for kube-apiserver for cluster %v: %v", cluster.Name, ip)
	if err := lcc.writeClusterEndpoint(cluster, ip); err != nil {
		return "", err
	}
	return ip, nil
}

func (lcc *LinodeClusterClient) writeClusterEndpoint(cluster *clusterv1.Cluster, ip string) error {
	glog.Infof("Updating cluster endpoint %v: %v.\n", cluster.Name, ip)
	cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{{
		Host: ip,
		Port: 6443,
	}}
	return lcc.client.Status().Update(context.TODO(), cluster)
}

/*
 * reconcileAddonsAndConfigmaps deploys kube-proxy and coredns addons, an
 * initial bootstrap token, kubeadm config, and some additional resources
 * This is done by executing the following commands:
 *
 *   export ka='kubeadm --kubeconfig <config>'
 *   $ka init phase bootstrap-token
 *   $ka init phase addon kube-proxy --apiserver-advertise-address <lb-IP-address> --pod-network-cidr 10.2.0.0/16
 *   $ka init phase upload-config kubeadm
 *   $ka init phase addon coredns --service-cidr 10.128.0.0/16
 */
func (lcc *LinodeClusterClient) reconcileAddonsAndConfigmaps(cluster *clusterv1.Cluster, version Version, ip string, lkeClient client.Client) error {
	glog.Infof("Reconciling XXX resources for cluster %v.", cluster.Name)

	if version.K8S() != "v1.13" {
		return fmt.Errorf("[%v] version %s unsupported", cluster.Name, version)
	}

	if checkDaemonset(lkeClient, "kube-system", "kube-proxy") {
		glog.Infof("Cluster %v already has reconcileAddonsAndConfigmaps", cluster.Name)
		return nil
	}

	kubeconfig, err := tempKubeconfig(lcc.client, cluster.Name)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig)

	if _, err := system("kubeadm --kubeconfig %s init phase bootstrap-token", kubeconfig); err != nil {
		return err
	}

	if _, err := system("kubeadm --kubeconfig %s init phase addon kube-proxy --apiserver-advertise-address %s --pod-network-cidr 10.2.0.0/16", kubeconfig, ip); err != nil {
		return err
	}

	if _, err := system("kubeadm --kubeconfig %s init phase upload-config kubeadm", kubeconfig); err != nil {
		return err
	}

	if _, err := system("kubeadm --kubeconfig %s init phase addon coredns --service-cidr 10.128.0.0/16", kubeconfig); err != nil {
		return err
	}

	return nil
}

// creates a new client for LKE
func lkeClientNew(cpcClient client.Client, cluster string) (client.Client, error) {
	kubeconfig, err := tempKubeconfig(cpcClient, cluster)
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return client.New(config, client.Options{})
}

func checkDaemonset(client client.Client, ns, name string) bool {
	daemonset := &appsv1.DaemonSet{}
	nn := types.NamespacedName{Namespace: ns, Name: name}
	if err := client.Get(context.Background(), nn, daemonset); err != nil {
		return false
	}

	// simple: exists, then ok
	return true
}

func (lcc *LinodeClusterClient) removeFinalizerFromSecret(clusterNamespace string, secretName string) error {
	// If there are no Machines, then we can remove the finalizers on the critical Secrets
	// NB: Don't block deletion (yet) for this reason. (Don't return Error)
	secret := &corev1.Secret{}
	if err := lcc.client.Get(context.Background(),
		types.NamespacedName{Namespace: clusterNamespace, Name: secretName},
		secret); err != nil {
		glog.Errorf("[%s] Could not get secret \"%s\" in order to remove finalizer. "+
			"Continuing with Cluster delete anyway", clusterNamespace, secretName)
		return nil
	}
	secret.Finalizers = []string{}
	if err := lcc.client.Update(context.Background(), secret); err != nil {
		glog.Errorf("[%s] Could not get secret \"%s\" in order to remove finalizer. "+
			"Continuing with Cluster delete anyway", clusterNamespace, secretName)
	}
	return nil
}

// Delete attempts to perform deletion for an LKE cluster.
//
// If the cluster should not be deleted, return an Error and cluster-api will
// requeue this Cluster for deletion.
func (lcc *LinodeClusterClient) Delete(cluster *clusterv1.Cluster) error {
	clusterNamespace := cluster.GetNamespace()
	glog.Infof("[%s] Attempting to delete this Cluster", clusterNamespace)

	// Delete the control plane Pod-creating resources including CCM (not
	// Secrets/ConfigMaps), so that we immediately prevent the Linode user from
	// adding additional resources to this Cluster.
	// TODO

	// List all Machines for this cluster. If any Machines exist for this cluster
	// we cannot delete it.
	machineList := &clusterv1.MachineList{}
	listOptions := client.InNamespace(cluster.GetNamespace())
	if err := lcc.client.List(context.Background(), listOptions, machineList); err != nil {
		errStr := fmt.Sprintf("[%s] Error deleting Cluster. Error listing Machines for cluster: %v", clusterNamespace, err)
		// Print the err that we return to cluster-api so that we can filter logs
		// using our prefix
		glog.Errorf(errStr)
		return fmt.Errorf(errStr)
	}

	if len(machineList.Items) > 0 {
		return fmt.Errorf("[%s] Error deleting Cluster. "+
			"Delete all Machines associated with this cluster", clusterNamespace)
	}

	// If no Machines remain then we can remove the finalizers from the critical Secrets
	if err := lcc.removeFinalizerFromSecret(clusterNamespace, "linode"); err != nil {
		glog.Errorf("[%s] Error removing finalizer from secret \"%s\": %s"+
			"Continuing with Cluster delete anyway", clusterNamespace, "linode", err)
	}

	if err := lcc.removeFinalizerFromSecret(clusterNamespace, "linode-ca"); err != nil {
		glog.Errorf("[%s] Error removing finalizer from secret \"%s\": %s"+
			"Continuing with Cluster delete anyway", clusterNamespace, "linode-ca", err)
	}

	// Delete our own namespace to clean everything else up
	clusterNamespaceObject := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterNamespace,
		},
	}
	if err := lcc.client.Delete(context.Background(), clusterNamespaceObject); err != nil {
		return fmt.Errorf("[%s] Error deleting Cluster namespace while deleting cluster: %s",
			clusterNamespace, err)
	}

	return nil
}
