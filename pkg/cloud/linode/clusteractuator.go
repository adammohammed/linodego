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

	"github.com/golang/glog"
	"golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	glog.Infof("Reconciling cluster %v.", cluster.Name)
	if err := lcc.reconcileControlPlane(cluster); err != nil {
		return err
	}
	return nil
}

// Validate that control plane services are deployed and running with expected configuration
// If they are not, deploy or modify them
func (lcc *LinodeClusterClient) reconcileControlPlane(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling control plane for cluster %v.", cluster.Name)

	if err := lcc.reconcileAPIServerService(cluster); err != nil {
		return err
	}

	if err := lcc.generateSecrets(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileAPIServer(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileEtcd(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileControllerManager(cluster); err != nil {
		return err
	}

	if err := lcc.reconcileScheduler(cluster); err != nil {
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileAPIServerService(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling API Server for cluster %v.", cluster.Name)
	// TODO: validate that API Server has an endpoint and return one if it does

	// TODO: Use Ingress for this! Don't provision a NodeBalancer per LKE cluster
	// Deploy a LoadBalancer service for the Cluster's API Server
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(apiserverServiceChartPath, cluster.Name, values); err != nil {
		return fmt.Errorf("Error reconciling apiserver service for cluster %v: %v", cluster.Name, err)
	}

	// Get the hostname or IP address of the LoadBalancer
	apiserverService := &corev1.Service{}
	err := lcc.client.Get(context.Background(),
		types.NamespacedName{Namespace: cluster.GetNamespace(), Name: "kube-apiserver"},
		apiserverService)
	if err != nil {
		return fmt.Errorf("Could not find kube-apiserver Service for cluster %v", cluster.Name)
	}
	glog.Infof("Found service for kube-apiserver for cluster %v: %v", cluster.Name, apiserverService.Name)
	if len(apiserverService.Status.LoadBalancer.Ingress) < 1 {
		return fmt.Errorf("No ExternalIPs yet for kube-apiserver for cluster %v", cluster.Name)
	}
	ip := apiserverService.Status.LoadBalancer.Ingress[0].IP

	// Write that NodeBalancer address as the cluster API endpoint
	glog.Infof("External IP for kube-apiserver for cluster %v: %v", cluster.Name, ip)
	if err := lcc.writeClusterEndpoint(cluster, ip); err != nil {
		return err
	}
	return nil
}

func (lcc *LinodeClusterClient) writeClusterEndpoint(cluster *clusterv1.Cluster, ip string) error {
	glog.Infof("Updating cluster endpoint %v: %v.\n", cluster.Name, ip)
	cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{{
		Host: ip,
		Port: 6443,
	}}
	if err := lcc.client.Update(context.Background(), cluster); err != nil {
		return err
	}
	return nil
}

func (lcc *LinodeClusterClient) reconcileAPIServer(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling API Server for cluster %v.", cluster.Name)
	// TODO: validate that API Server is running for the cluster

	// Deploy API Server for the LKE cluster
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(apiserverChartPath, cluster.Name, values); err != nil {
		glog.Errorf("Error reconciling apiserver for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileEtcd(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling etcd for cluster %v.", cluster.Name)
	// TODO: validate that etcd is running for the cluster

	// Deploy etcd for the LKE cluster
	values := make(map[string]interface{})
	if err := lcc.chartDeployer.DeployChart(etcdChartPath, cluster.Name, values); err != nil {
		glog.Errorf("Error reconciling etcd for cluster %v: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileControllerManager(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling kube-controller-manager for cluster %v.", cluster.Name)
	// TODO: validate that kube-controller-manager is running for the cluster

	// Deploy the controller-manager for the LKE cluster
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(cmChartPath, cluster.Name, values); err != nil {
		glog.Errorf("Error reconciling kube-controller-manager for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) reconcileScheduler(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling scheduler for cluster %v.", cluster.Name)
	// TODO: validate that scheduler is running for the cluster
	// Dont re-deploy the scheduler if we don't need to

	// Deploy the scheduler for the LKE cluster
	values := map[string]interface{}{
		"ClusterName": cluster.Name,
	}

	if err := lcc.chartDeployer.DeployChart(schedChartPath, cluster.Name, values); err != nil {
		glog.Errorf("Error reconciling scheduler for cluster %v: %v", cluster.Name, err)

		return err
	}

	return nil
}

func (lcc *LinodeClusterClient) Delete(cluster *clusterv1.Cluster) error {
	glog.Infof("Deleting cluster %v.", cluster.Name)
	return nil
}
