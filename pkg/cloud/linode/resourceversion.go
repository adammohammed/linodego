/*
Copyright 2019 Linode, LLC.

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
	"strings"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TreatMeAsUptodate = "TreatMeAsUptodate"

func getServiceMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &corev1.Service{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getDeploymentMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &appsv1.Deployment{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getRoleMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &v1beta1.Role{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getRolebindingMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &v1beta1.RoleBinding{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getConfigmapMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &corev1.ConfigMap{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getDaemonsetMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &appsv1.DaemonSet{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getStatefulsetMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &appsv1.StatefulSet{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getServiceaccountMeta(client client.Client, namespace, name string) *metav1.ObjectMeta {
	x := &corev1.ServiceAccount{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getClusterroleMeta(client client.Client, name string) *metav1.ObjectMeta {
	x := &v1beta1.ClusterRole{}
	nn := types.NamespacedName{Namespace: "", Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

func getClusterrolebindingMeta(client client.Client, name string) *metav1.ObjectMeta {
	x := &v1beta1.ClusterRoleBinding{}
	nn := types.NamespacedName{Namespace: "", Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}

/*
XXX: what API to use?
func getCustomresourcedefinitionMeta(client client.Client, name string) *metav1.ObjectMeta {
	x := &apiext_v1beta1.CustomResourceDefinition{}
	nn := types.NamespacedName{Namespace: "", Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		glog.Infof("WOO can't get crd %s: %v", name, err)
		return nil
	}
	return &x.ObjectMeta
}
*/

/*
XXX: what API to use?
func getCsidriverMeta(client client.Client, name string) *metav1.ObjectMeta {
	x := &storage_v1beta1.CSIDriver{}
	nn := types.NamespacedName{Namespace: "", Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}
*/

/*
XXX: what API to use?
func getStorageclassMeta(client client.Client, name string) *metav1.ObjectMeta {
	x := &storage_v1beta1.StorageClass{}
	nn := types.NamespacedName{Namespace: "", Name: name}
	if err := client.Get(context.Background(), nn, x); err != nil {
		return nil
	}
	return &x.ObjectMeta
}
*/

func getResourceVersion(client client.Client, namespace string, r *Resource) (string, error) {

	var meta *metav1.ObjectMeta

	switch strings.ToLower(r.Kind) {
	case "service":
		meta = getServiceMeta(client, namespace, r.Name)
	case "deployment":
		meta = getDeploymentMeta(client, namespace, r.Name)
	case "role":
		meta = getRoleMeta(client, namespace, r.Name)
	case "rolebinding":
		meta = getRolebindingMeta(client, namespace, r.Name)
	case "configmap":
		meta = getConfigmapMeta(client, namespace, r.Name)
	case "daemonset":
		meta = getDaemonsetMeta(client, namespace, r.Name)
	case "statefulset":
		meta = getStatefulsetMeta(client, namespace, r.Name)
	case "serviceaccount":
		meta = getServiceaccountMeta(client, namespace, r.Name)
	case "clusterrole":
		meta = getClusterroleMeta(client, r.Name)
	case "clusterrolebinding":
		meta = getClusterrolebindingMeta(client, r.Name)
	case "customresourcedefinition":
		return TreatMeAsUptodate, nil
	//	XXX meta = getCustomresourcedefinitionMeta(client, r.Name)
	case "csidriver":
		return TreatMeAsUptodate, nil
	//	XXX meta = getCsidriverMeta(client, r.Name)
	case "storageclass":
		return TreatMeAsUptodate, nil
	//meta = XXX getStorageclassMeta(client, r.Name)
	default:
		return "", fmt.Errorf("can't get meta-information for resource kind %s", r.Kind)
	}

	if meta == nil {
		return "", nil
	}

	version := meta.Annotations["lke.linode.com/caplke-version"]
	glog.Infof("resource %s/%s of kind %s exists, version=%s", namespace, r.Name, r.Kind, version)

	return version, nil
}
