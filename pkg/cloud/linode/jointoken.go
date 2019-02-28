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
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	joinTokenSecretName = "kubeadm-join-token"
)

func getJoinToken(client client.Client, cluster *clusterv1.Cluster) (string, error) {
	joinTokenSecret, err := getJoinTokenSecret(client, cluster)

	// If no join token exists for the cluster, generate one
	if errors.IsNotFound(err) {
		return generateJoinToken(client, cluster)
	} else if err != nil {
		return "", fmt.Errorf(
			"Error retrieving join token secret for cluster (%v): %v",
			cluster.Name, err)
	}

	// If a join token does exist and it's not expired, return it
	if !joinTokenExpired(joinTokenSecret) {
		return string(joinTokenSecret.Data["token"]), nil
	}

	// If a join token exists and is expired, generate one
	return generateJoinToken(client, cluster)
}

func getJoinTokenSecret(client client.Client, cluster *clusterv1.Cluster) (*corev1.Secret, error) {
	// Look for a join token secret in the namespace of the Cluster object.
	joinTokenSecret := &corev1.Secret{}
	if err := client.Get(context.Background(),
		types.NamespacedName{Namespace: cluster.GetNamespace(), Name: joinTokenSecretName},
		joinTokenSecret); err != nil {
		return nil, err
	}
	return joinTokenSecret, nil
}

func generateJoinToken(client client.Client, cluster *clusterv1.Cluster) (string, error) {
	adminKubeconfig = getAdminKubeconfig(clusterNamespace)

	# no admin Kubeconfig, create a random join token to be used by the master machine init script
	if not adminKubeconfig:
		return generateJoinTokenForMachineMaster(clusterNamespace)

	return generateJoinTokenForCPCMaster(clusterNamespace)

	// If one isn't found, create one.
	joinToken, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		glog.Errorf("Unable to create kubeadm join token: %v", err)
		return "", err
	}
	joinTokenSecret.ObjectMeta = metav1.ObjectMeta{
		Namespace: cluster.GetNamespace(),
		Name:      joinTokenSecretName,
	}
	joinTokenSecret.Type = corev1.SecretTypeOpaque
	joinTokenSecret.Data = map[string][]byte{
		"token": []byte(joinToken),
	}
	err = client.Create(context.Background(), joinTokenSecret)
	if err != nil {
		return "", fmt.Errorf("error creating join token secret for cluster")
	}
	return "", nil
}
