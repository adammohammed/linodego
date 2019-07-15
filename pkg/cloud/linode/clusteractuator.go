/*
Copyright 2018 Linode, LLC.
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
	"os"

	"golang.org/x/net/context"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	chartPath                 = "charts"
	lkeclusterPath            = chartPath + "/" + "lkecluster"
	etcdChartPath             = lkeclusterPath + "/" + "etcd"
	apiserverServiceChartPath = lkeclusterPath + "/" + "apiserver-service"
	apiserverChartPath        = lkeclusterPath + "/" + "apiserver"
	cmChartPath               = lkeclusterPath + "/" + "controller-manager"
	schedChartPath            = lkeclusterPath + "/" + "scheduler"

	kubeletResourcesPath = lkeclusterPath + "/" + "kubelet-resources"
	cniResourcesPath     = lkeclusterPath + "/" + "cni"
	ccmChartPath         = lkeclusterPath + "/" + "ccm"

	csiResourcePath = lkeclusterPath + "/" + "csi/lke"

	wgPath                 = lkeclusterPath + "/" + "wg"
	wgLKECredsResourcePath = wgPath + "/" + "lke/clusterroles"
)

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

func (lcc *LinodeClusterClient) Reconcile(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling cluster %v.", cluster.Name)
	if err := lcc.reconcileControlPlane(cluster); err != nil {
		return err
	}
	return nil
}

// Validate that control plane services are deployed and running with expected configuration
// If they are not, deploy or modify them
func (lcc *LinodeClusterClient) reconcileControlPlane(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling control plane for cluster %v.", cluster.Name)

	ip, err := lcc.reconcileAPIServerService(cluster)
	if err != nil {
		return err
	}

	if err := lcc.generateSecrets(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileEtcd(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileAPIServer(cluster, ip); err != nil {
		return err
	}

	if err := lcc.reconcileControllerManager(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileScheduler(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileKubeletResources(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileAddonsAndConfigmaps(cluster, ip); err != nil {
		return err
	}

	if err := lcc.reconcileCNI(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileCCM(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileCSI(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileWG(cluster); err != nil {
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileAPIServerService(cluster *clusterv1.Cluster) (string, error) {
	klog.Infof("Reconciling API Server for cluster %v.", cluster.Name)
	// TODO: validate that API Server has an endpoint and return one if it does

	// TODO: Use Ingress for this! Don't provision a NodeBalancer per LKE cluster
	// Deploy a LoadBalancer service for the Cluster's API Server
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(apiserverServiceChartPath, cluster.Name, values); err != nil {
		return "", fmt.Errorf("Error reconciling apiserver service for cluster %v: %v", cluster.Name, err)
	}

	// Get the hostname or IP address of the LoadBalancer
	apiserverService := &corev1.Service{}
	err := lcc.client.Get(context.Background(),
		types.NamespacedName{Namespace: cluster.GetNamespace(), Name: "kube-apiserver"},
		apiserverService)
	if err != nil {
		return "", fmt.Errorf("Could not find kube-apiserver Service for cluster %v", cluster.Name)
	}
	klog.Infof("Found service for kube-apiserver for cluster %v: %v", cluster.Name, apiserverService.Name)
	if len(apiserverService.Status.LoadBalancer.Ingress) < 1 {
		return "", fmt.Errorf("No ExternalIPs yet for kube-apiserver for cluster %v", cluster.Name)
	}
	ip := apiserverService.Status.LoadBalancer.Ingress[0].IP

	// Write that NodeBalancer address as the cluster API endpoint
	klog.Infof("External IP for kube-apiserver for cluster %v: %v", cluster.Name, ip)
	if err := lcc.writeClusterEndpoint(cluster, ip); err != nil {
		return "", err
	}
	return ip, nil
}

func (lcc *LinodeClusterClient) writeClusterEndpoint(cluster *clusterv1.Cluster, ip string) error {
	klog.Infof("Updating cluster endpoint %v: %v.\n", cluster.Name, ip)
	cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{{
		Host: ip,
		Port: 6443,
	}}
	return lcc.client.Status().Update(context.TODO(), cluster)
}

func (lcc *LinodeClusterClient) reconcileAPIServer(cluster *clusterv1.Cluster, ip string) error {
	klog.Infof("Reconciling API Server for cluster %v.", cluster.Name)
	// TODO: validate that API Server is running for the cluster

	// Deploy API Server for the LKE cluster
	values := map[string]interface{}{
		"ClusterName":      cluster.Name,
		"AdvertiseAddress": ip,
	}

	if err := lcc.chartDeployer.DeployChart(apiserverChartPath, cluster.Name, values); err != nil {
		klog.Errorf("Error reconciling apiserver for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileEtcd(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling etcd for cluster %v.", cluster.Name)
	// TODO: validate that etcd is running for the cluster

	secret := &corev1.Secret{}
	name := types.NamespacedName{Namespace: "kube-system", Name: "linode"}
	if err := lcc.client.Get(context.Background(), name, secret); err != nil {
		return err
	}

	// Deploy etcd for the LKE cluster
	values := map[string]interface{}{
		// StorePrefix example: us-east/cpc1190/12ahd312/lke123123
		"StorePrefix": fmt.Sprintf("%s/cpc%s/%s/%s",
			secret.Data["region"],
			secret.Data["id"],
			secret.Data["timestamp"],
			cluster.Name),
	}
	if err := lcc.chartDeployer.DeployChart(etcdChartPath, cluster.Name, values); err != nil {
		klog.Errorf("Error reconciling etcd for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileControllerManager(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling kube-controller-manager for cluster %v.", cluster.Name)
	// TODO: validate that kube-controller-manager is running for the cluster

	// Deploy the controller-manager for the LKE cluster
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(cmChartPath, cluster.Name, values); err != nil {
		klog.Errorf("Error reconciling kube-controller-manager for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileScheduler(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling scheduler for cluster %v.", cluster.Name)
	// TODO: validate that scheduler is running for the cluster
	// Dont re-deploy the scheduler if we don't need to

	// Deploy the scheduler for the LKE cluster
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(schedChartPath, cluster.Name, values); err != nil {
		klog.Errorf("Error reconciling scheduler for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileKubeletResources(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling kubelet resources for cluster %v.", cluster.Name)

	chartDeployerLKE, err := newChartDeployerLKE(lcc.client, cluster.Name)
	if err != nil {
		klog.Errorf("Error creating new chartDeployerLKE for cluster %v: %v", cluster.Name, err)
		return err
	}

	if err := chartDeployerLKE.DeployChart(kubeletResourcesPath, "kube-system", map[string]interface{}{}); err != nil {
		klog.Errorf("Error reconciling kubelet resources for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
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
func (lcc *LinodeClusterClient) reconcileAddonsAndConfigmaps(cluster *clusterv1.Cluster, ip string) error {
	klog.Infof("Reconciling kubelet resources for cluster %v.", cluster.Name)

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

func (lcc *LinodeClusterClient) reconcileCNI(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling CNI for cluster %v.", cluster.Name)

	chartDeployerLKE, err := newChartDeployerLKE(lcc.client, cluster.Name)
	if err != nil {
		klog.Errorf("Error creating new chartDeployerLKE for cluster %v: %v", cluster.Name, err)
		return err
	}

	if err := chartDeployerLKE.DeployChart(cniResourcesPath, "kube-system", map[string]interface{}{}); err != nil {
		klog.Errorf("Error reconciling CNI for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileCCM(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling CCM for cluster %v.", cluster.Name)

	values := map[string]interface{}{}

	if err := lcc.chartDeployer.DeployChart(ccmChartPath, cluster.Name, values); err != nil {
		klog.Errorf("Error reconciling CCM for cluster %v: %v", cluster.Name, err)

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

// copies the 'kube-system-<cluster>/linode' secret in CPC to the
// 'kube-system/linode' secret in LKE cluster <cluster>
func copyLinodeSecret(cpcClient, lkeClient client.Client, namespace string) error {

	secret := &corev1.Secret{}
	if err := cpcClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "linode"}, secret); err != nil {
		return err
	}

	lkeSecret := &corev1.Secret{}
	lkeSecret.ObjectMeta = metav1.ObjectMeta{
		Namespace: "kube-system",
		Name:      "linode",
	}
	if err := lkeClient.Get(context.Background(), types.NamespacedName{Namespace: "kube-system", Name: "linode"}, lkeSecret); err == nil {
		return nil
	}

	lkeSecret.Type = corev1.SecretTypeOpaque
	lkeSecret.Data = secret.Data // note: not a deep copy
	return lkeClient.Create(context.Background(), lkeSecret)
}

func (lcc *LinodeClusterClient) reconcileCSI(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling CSI for cluster %v.", cluster.Name)

	chartDeployerLKE, err := newChartDeployerLKE(lcc.client, cluster.Name)
	if err != nil {
		klog.Errorf("Error creating new chartDeployerLKE for cluster %v: %v", cluster.Name, err)
		return err
	}

	lkeClient, err := lkeClientNew(lcc.client, cluster.Name)
	if err != nil {
		return err
	}

	// Copy linode secret from CPC to LKE
	if err := copyLinodeSecret(lcc.client, lkeClient, clusterNamespace(cluster.Name)); err != nil {
		klog.Errorf("Error creating a linode secret in the LKE %v: %v", cluster.Name, err)
		return err
	}

	values := map[string]interface{}{}

	if err := chartDeployerLKE.DeployChart(csiResourcePath, "kube-system", values); err != nil {
		klog.Errorf("Error reconciling CSI for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileWG(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling WG controller for cluster %v.", cluster.Name)

	chartDeployerLKE, err := newChartDeployerLKE(lcc.client, cluster.Name)
	if err != nil {
		klog.Errorf("Error creating new chartDeployerLKE for cluster %v: %v", cluster.Name, err)
		return err
	}

	values := map[string]interface{}{}

	if err := chartDeployerLKE.DeployChart(wgLKECredsResourcePath, "kube-system", values); err != nil {
		klog.Errorf("Error reconciling WG credentials for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) Delete(cluster *clusterv1.Cluster) error {
	klog.Infof("Deleting cluster %v.", cluster.Name)
	return nil
}
