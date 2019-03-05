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
	//"golang.org/x/net/context"
	//corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/types"
	//clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getJoinToken(client client.Client, cluster string) (string, error) {

	/*
		API implementation:

		List all secrets of type "bootstrap.kubernetes.io/token"
		Delete each secret which is expired (data.expiration < current time)
		If there are no secrets left, then create one (the easiest way is still to use `kubeadm --kubeconfig <config> token create`)
	*/

	/*
		Kubeadm implementation:

		Delete '<invalid>' tokens:
		kubeadm --kubeconfig <config> token list | awk '$2 == "<invalid>" { system("kubeadm --kubeconfig <config> token delete " $1) }'

		Gimme first non-expired token:
		kubeadm --kubeconfig <config> token list | awk 'NR>1 && !($2=="<invalid>") {print $1; exit}'

		If empty, then
		kubeadm --kubeconfig <config> token create
	*/

	return "", fmt.Errorf("KRISPY")

	/*
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

	*/
}
