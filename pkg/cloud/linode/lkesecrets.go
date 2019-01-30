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
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"os/exec"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createOpaqueSecret(client client.Client, name, namespace string, data map[string][]byte) error {
	secret := &corev1.Secret{}

	secret.ObjectMeta = metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data

	return client.Create(context.Background(), secret)
}

type certsInit = struct {
	dir string /* directory containing certs, say, "/tmp/<cluster>/pki" */
}

func run(prog string, args ...string) (string, error) {

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(prog, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if outStr != "" {
		// we can print stdout here
	}
	if errStr != "" {
		// we can print stderr here
	}

	return outStr, err
}

func getKubeadmConfig(client client.Client, cluster *clusterv1.Cluster) ([]byte, error) {

	// I am the vine, ye are the branches: He that abideth in me, and I in him,
	// the same bringeth forth much fruit: for without me ye can do nothing
	return nil, nil

}

func createKubeadmFile(client client.Client, dirname string, cluster *clusterv1.Cluster) (string, error) {

	filename := dirname + "/" + "kubeadm.conf"

	if data, err := getKubeadmConfig(client, cluster); err != nil {
		return "", err
	} else if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return "", err
	}

	return filename, nil

}

func generateCertsInit(client client.Client, cluster *clusterv1.Cluster) (*certsInit, error) {

	dirname := "/tmp/" + cluster.Name + "/pki"
	if err := os.MkdirAll(dirname, os.ModePerm); err != nil {
		return nil, err
	}

	if config, err := createKubeadmFile(client, dirname, cluster); err != nil {
		return nil, err
	} else if _, err := run("kubeadm", "init", "phase", "certs", "all", "--config", config); err != nil {
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

func generateCerts(client client.Client, cluster *clusterv1.Cluster) (map[string][]byte, map[string][]byte, error) {

	init, err := generateCertsInit(client, cluster)
	if err != nil {
		return nil, nil, err
	}
	defer generateCertsFini(init)

	k8sCerts := make(map[string][]byte)
	k8skeyval := map[string]string{
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
	if err := addFiles(init, k8sCerts, k8skeyval); err != nil {
		return nil, nil, err
	}

	etcdCerts := make(map[string][]byte)
	etcdKeyval := map[string]string{
		"ca.crt":                 "etcd/",
		"ca.key":                 "etcd/",
		"healthcheck-client.crt": "etcd/",
		"healthcheck-client.key": "etcd/",
		"peer.crt":               "etcd/",
		"peer.key":               "etcd/",
		"server.crt":             "etcd/",
		"server.key":             "etcd/",
	}
	if err := addFiles(init, etcdCerts, etcdKeyval); err != nil {
		return nil, nil, err
	}

	return k8sCerts, etcdCerts, err
}

func generateCertSecrets(client client.Client, cluster *clusterv1.Cluster) error {

	k8sCerts, etcdCerts, err := generateCerts(client, cluster)
	if err != nil {
		return err
	}

	ns := cluster.GetNamespace()

	if err := createOpaqueSecret(client, "k8s-certs", ns, k8sCerts); err != nil {
		return err
	}

	if err := createOpaqueSecret(client, "etcd-certs", ns, etcdCerts); err != nil {
		return err
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
	return createOpaqueSecret(client, name, cluster.GetNamespace(), data)
}

/*
 * create secrets needed for operation of control plane components
 */
func (lcc *LinodeClusterClient) generateSecrets(cluster *clusterv1.Cluster) error {

	glog.Infof("Creating secrets for cluster %v.", cluster.Name)

	if err := generateCertSecrets(lcc.client, cluster); err != nil {
		return err
	}

	if err := generateNodeWatcherSecrets(lcc.client, cluster); err != nil {
		return err
	}

	return nil
}
