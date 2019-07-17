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
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"k8s.io/klog"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createOpaqueSecret(client client.Client, namespace, name string, data map[string][]byte) error {
	testSecret := &corev1.Secret{}
	client.Get(context.Background(),
		types.NamespacedName{Namespace: namespace, Name: name},
		testSecret)
	if len(testSecret.Name) > 0 {
		klog.Infof("Not writing a secret that already exists")
		return nil
	}

	secret := &corev1.Secret{}

	secret.ObjectMeta = metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data

	return client.Create(context.Background(), secret)
}

/* Temporarily holds PKI data for a cluster */
type certsInit = struct {
	/* directory containing certs, e.g. "/tmp/<cluster>/pki" */
	dir string
}

func run(prog string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(prog, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if outStr != "" {
		klog.Infof("stdout:\n%v", outStr)
	}
	if errStr != "" {
		klog.Infof("stderr:\n%v", errStr)
	}

	return strings.TrimSpace(outStr), err
}

type kubeadmConfigParams struct {
	NodeBalancerHostname string
	ClusterName          string
	CertsDir             string
	KubeconfigDir        string
}

const kubeadmConfigTemplate = `kind: ClusterConfiguration
apiVersion: kubeadm.k8s.io/v1beta1
apiServer:
  certSANs:
  - {{ .NodeBalancerHostname }}
  - kube-apiserver.kube-system-{{ .ClusterName }}.svc.cluster.local
  - localhost
  extraArgs:
    authorization-mode: Node,RBAC
    cloud-provider: external
    feature-gates: CSINodeInfo=true,CSIDriverRegistry=true,BlockVolume=true,CSIBlockVolume=true
  timeoutForControlPlane: 4m0s
certificatesDir: {{ .CertsDir }}
clusterName: {{ .ClusterName }}
controlPlaneEndpoint: ""
controllerManager:
  extraArgs:
    cloud-provider: external
    feature-gates: ""
dns:
  type: CoreDNS
etcd:
  local:
    dataDir: /var/lib/etcd
    serverCertSANs:
      - etcd
      - etcd.kube-system-{{ .ClusterName }}.svc.cluster.local
    peerCertSANs:
      - etcd
      - etcd.kube-system-{{ .ClusterName }}.svc.cluster.local
imageRepository: k8s.gcr.io
kubernetesVersion: v1.13.3
networking:
  dnsDomain: cluster.local
  podSubnet: 10.2.0.0/16
  serviceSubnet: 10.128.0.0/16
scheduler: {}
`

func getKubeadmConfig(client client.Client, cluster *clusterv1.Cluster, dirname string) ([]byte, error) {
	if len(cluster.Status.APIEndpoints) < 1 {
		return nil, fmt.Errorf("No APIEndpoints while writing certs for cluster (LoadBalancer Service not provisioned?) %v", cluster.Name)
	}

	configParams := kubeadmConfigParams{
		NodeBalancerHostname: cluster.Status.APIEndpoints[0].Host,
		ClusterName:          cluster.Name,
		CertsDir:             dirname,
	}

	tmpl, err := template.New("kubeadm-config").Parse(kubeadmConfigTemplate)
	if err != nil {
		return nil, err
	}

	var configBuf bytes.Buffer
	if err := tmpl.Execute(&configBuf, configParams); err != nil {
		return nil, err
	}

	return configBuf.Bytes(), nil
}

func createKubeadmFile(client client.Client, dirname string, cluster *clusterv1.Cluster) (string, error) {
	filename := dirname + "/" + "kubeadm.conf"

	fmt.Printf(filename)

	if data, err := getKubeadmConfig(client, cluster, dirname); err != nil {
		return "", err
	} else if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return "", err
	}

	return filename, nil
}

func patchKubeconfig(path, address, port string) error {
	expr := fmt.Sprintf(`s,\(server: https://\)\(.*\)$,\1%s:%s,`, address, port)
	_, err := run("sed", "-e", expr, "-i", path)
	return err
}

func generateCertsInit(client client.Client, cluster *clusterv1.Cluster) (*certsInit, error) {
	dirname := "/tmp/" + cluster.Name + "/pki"
	if err := os.MkdirAll(dirname, os.ModePerm); err != nil {
		return nil, err
	}

	// Generate PKI material with the `kubeadm init phase certs` command
	if config, err := createKubeadmFile(client, dirname, cluster); err != nil {
		return nil, err
	} else if _, err := run("kubeadm", "init", "phase", "certs", "all", "--config", config); err != nil {
		return nil, err
	}

	// Don't walk up the directory tree to place the kubeconfigs, keep
	// things rooted at the specified directory
	kubeconfigDir := dirname + "/kubeconfigs"

	// Generate client kubeconfigs with the `kubeadm init phase kubeconfig` command
	if _, err := run("kubeadm", "init", "phase", "kubeconfig", "all",
		"--kubeconfig-dir", kubeconfigDir,
		"--cert-dir", dirname,
		"--apiserver-advertise-address", cluster.Status.APIEndpoints[0].Host); err != nil {
		return nil, err
	}

	// set proper domain names in kubeconfigs

	apiServer := fmt.Sprintf("kube-apiserver.%s.svc.cluster.local", clusterNamespace(cluster.Name))

	if err := patchKubeconfig(kubeconfigDir+"/"+"controller-manager.conf", apiServer, "6443"); err != nil {
		return nil, err
	}
	if err := patchKubeconfig(kubeconfigDir+"/"+"scheduler.conf", apiServer, "6443"); err != nil {
		return nil, err
	}

	return &certsInit{dir: dirname}, nil
}

func generateCertsFini(init *certsInit) {
	if dir, err := ioutil.ReadDir(init.dir); err == nil {
		for _, d := range dir {
			os.RemoveAll(init.dir + "/" + d.Name())
		}
	}
}

/*
 * Add the contents of a dir/subpath/file to the certs mapping.
 *
 * Example 1:
 *     addFile(init, certs, "apiserver-etcd-client.crt", "")
 * does this (in pseudo-code):
 *     certs["apiserver-etcd-client.crt"] = $(cat /etc/lke0/pki/apiserver-etcd-client.crt)
 *
 * Example 2:
 *     addFile(init, certs, "ca.crt", "etcd/")
 * does this (in pseudo-code):
 *     certs["ca.crt"] = $(cat /etc/lke0/pki/etcd/ca.crt)
 */
func addFile(init *certsInit, certs map[string][]byte, file, subpath string) error {
	data, err := ioutil.ReadFile(init.dir + "/" + subpath + "/" + file)
	if err != nil {
		return err
	}
	certs[file] = data
	return nil
}

func addFiles(init *certsInit, certs map[string][]byte, keyval map[string]string) error {
	for key, value := range keyval {
		err := addFile(init, certs, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

/* Add a Kubeconfig file that we find using :init: and :kubeconfigFilename: to
 * a new secret map with :secretName:. Store this secret map in :kubeconfigs: */
func addKubeconfig(kubeconfigs map[string]map[string][]byte, secretName string, kubeconfigFilename string, init *certsInit) error {
	kubeconfigs[secretName] = make(map[string][]byte)
	kubeconfigPaths := map[string]string{kubeconfigFilename: "kubeconfigs/"}
	if err := addFiles(init, kubeconfigs[secretName], kubeconfigPaths); err != nil {
		return err
	}
	return nil
}

func generateCerts(client client.Client, cluster *clusterv1.Cluster) (map[string][]byte, map[string][]byte, map[string]map[string][]byte, error) {
	init, err := generateCertsInit(client, cluster)
	if err != nil {
		return nil, nil, nil, err
	}
	defer generateCertsFini(init)

	k8sCerts := make(map[string][]byte)
	k8sPaths := map[string]string{
		"apiserver-etcd-client.crt":    "",
		"apiserver-etcd-client.key":    "",
		"apiserver-kubelet-client.crt": "",
		"apiserver-kubelet-client.key": "",
		"apiserver.crt":                "",
		"apiserver.key":                "",
		"ca.crt":                       "",
		"ca.key":                       "",
		"front-proxy-ca.crt":           "",
		"front-proxy-ca.key":           "",
		"front-proxy-client.crt":       "",
		"front-proxy-client.key":       "",
		"sa.key":                       "",
		"sa.pub":                       "",
	}
	if err := addFiles(init, k8sCerts, k8sPaths); err != nil {
		return nil, nil, nil, err
	}

	etcdCerts := make(map[string][]byte)
	etcdPaths := map[string]string{
		"ca.crt":                 "etcd/",
		"ca.key":                 "etcd/",
		"healthcheck-client.crt": "etcd/",
		"healthcheck-client.key": "etcd/",
		"peer.crt":               "etcd/",
		"peer.key":               "etcd/",
		"server.crt":             "etcd/",
		"server.key":             "etcd/",
	}
	if err := addFiles(init, etcdCerts, etcdPaths); err != nil {
		return nil, nil, nil, err
	}

	/* Each Kubeconfig is placed in a separate secret map, so that users
	 * can avoid using subPath and thus get live updates. To do this
	 * we used a nested map */
	kubeconfigs := make(map[string]map[string][]byte)
	if err := addKubeconfig(kubeconfigs, "admin-kubeconfig", "admin.conf", init); err != nil {
		return nil, nil, nil, err
	}
	if err := addKubeconfig(kubeconfigs, "controller-manager-kubeconfig", "controller-manager.conf", init); err != nil {
		return nil, nil, nil, err
	}
	if err := addKubeconfig(kubeconfigs, "scheduler-kubeconfig", "scheduler.conf", init); err != nil {
		return nil, nil, nil, err
	}
	if err := addKubeconfig(kubeconfigs, "kubelet-kubeconfig", "kubelet.conf", init); err != nil {
		return nil, nil, nil, err
	}

	return k8sCerts, etcdCerts, kubeconfigs, err
}

func generateCertSecrets(client client.Client, cluster *clusterv1.Cluster) error {
	k8sCerts, etcdCerts, kubeconfigs, err := generateCerts(client, cluster)
	if err != nil {
		return err
	}

	ns := cluster.GetNamespace()

	// Write secrets for the core k8s PKI material
	if err := createOpaqueSecret(client, ns, "k8s-certs", k8sCerts); err != nil {
		return err
	}

	// Write secrets for the etcd PKI material
	if err := createOpaqueSecret(client, ns, "etcd-certs", etcdCerts); err != nil {
		return err
	}

	// Write secrets for each of the client kubeconfigs that we generated for
	// the admin, controller-manager, scheduler, and kubelet
	for secretName, secretMap := range kubeconfigs {
		if err := createOpaqueSecret(client, ns, secretName, secretMap); err != nil {
			return err
		}
	}

	return nil
}

/*
 * create a string of form "<token>,wg-node-watcher,wg-node-watcher" where
 * <token> is a crypto-safe random (printable) string
 */
func generateNodeWatcherToken() ([]byte, error) {
	binToken := make([]byte, 32)
	if _, err := crand.Read(binToken); err != nil {
		return nil, err
	}

	token := hex.EncodeToString(binToken)
	return []byte(token + ",wg-node-watcher,wg-node-watcher"), nil
}

/*
 * create a secret containing a token for the wireguard watcher
 *
 *     apiVersion: v1
 *     data:
 *       wg-node-watcher-token: $WATCHER_TOKEN
 *     kind: Secret
 *     metadata:
 *       name: wg-node-watcher-token
 *       namespace: kube-system-$CLUSTER_NAME
 *
 */
func generateNodeWatcherSecrets(client client.Client, cluster *clusterv1.Cluster) error {
	token, err := generateNodeWatcherToken()
	if err != nil {
		return err
	}

	name := "wg-node-watcher-token"
	data := map[string][]byte{name: token}
	return createOpaqueSecret(client, cluster.GetNamespace(), name, data)
}

/*
 * create a secret containing credentials to access object storage
 *
 *     apiVersion: v1
 *     kind: Secret
 *     data:
 *       access: $access
 *       secret: $secret
 *       endpoint: $endpoint
 *     metadata:
 *       name: object-storage
 *       namespace: kube-system-$CLUSTER_NAME
 *
 */
func generateObjectStorageSecrets(client client.Client, cluster *clusterv1.Cluster) error {

	name := "object-storage"

	objStorageSecret := &corev1.Secret{}
	if err := client.Get(context.Background(),
		types.NamespacedName{Namespace: "kube-system", Name: name},
		objStorageSecret); err != nil {
		return err
	}

	return createOpaqueSecret(client, cluster.GetNamespace(), name, objStorageSecret.Data)
}

/*
 * create secrets needed for operation of control plane components
 */
func (lcc *LinodeClusterClient) generateSecrets(cluster *clusterv1.Cluster) error {
	klog.Infof("Creating secrets for cluster %v.", cluster.Name)

	if err := generateCertSecrets(lcc.client, cluster); err != nil {
		klog.Errorf("Error generating certs for cluster %v: %v.", cluster.Name, err)
		return err
	}

	if err := generateNodeWatcherSecrets(lcc.client, cluster); err != nil {
		klog.Errorf("Error generating NodeWatcher token for cluster %v: %v.", cluster.Name, err)
		return err
	}

	if err := generateObjectStorageSecrets(lcc.client, cluster); err != nil {
		klog.Errorf("Error generating ObjectStorage secrets for cluster %v: %v.", cluster.Name, err)
		return err
	}

	return nil
}
