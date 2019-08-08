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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var objSession = session.Must(session.NewSession())

// createSecret creates a secret with the given type in the given namespace.
func createSecret(client client.Client,
	secretType corev1.SecretType,
	namespace, name string,
	data map[string][]byte,
	overwrite bool,
	finalizer string,
) error {
	secret := &corev1.Secret{}

	secret.ObjectMeta = metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
	secret.Type = secretType
	secret.Data = data

	// write a finalizer if one is provided
	if finalizer != "" {
		// Add a finalizer. We can't allow this secret to be deleted until this secret
		// is used to clean up Cluster resources.
		secret.ObjectMeta.Finalizers = []string{finalizer}
	}

	testSecret := &corev1.Secret{}
	client.Get(context.Background(),
		types.NamespacedName{Namespace: namespace, Name: name},
		testSecret)
	if len(testSecret.Name) > 0 {
		if !overwrite {
			glog.Infof("[%s] Not writing a secret which already exists: %s", namespace, name)
			// Pass if the secret already exists and overwrite is false
			return nil
		}

		glog.Infof("[%s] We are replacing an existing secret: %s", namespace, name)
		if err := client.Delete(context.Background(), secret); err != nil {
			return err
		}
	}

	return client.Create(context.Background(), secret)
}

func createDockerSecret(
	client client.Client,
	namespace,
	name string,
	data map[string][]byte,
	overwrite bool,
	finalizer string,
) error {
	return createSecret(client, corev1.SecretTypeDockerConfigJson, namespace, name, data, overwrite, finalizer)
}

func createOpaqueSecret(
	client client.Client,
	namespace,
	name string,
	data map[string][]byte,
	overwrite bool,
	finalizer string,
) error {
	return createSecret(client, corev1.SecretTypeOpaque, namespace, name, data, overwrite, finalizer)
}

// generateObjectBucketName generates a bucket name of the form clusterName-rand where rand is a
// 4-byte hexadecimal string.
func generateObjectBucketName(clusterName string) (string, error) {
	suffixBytes := make([]byte, 4)

	if _, errRead := crand.Read(suffixBytes); errRead != nil {
		return "", errRead
	}

	suffix := hex.EncodeToString(suffixBytes)

	bucketName := fmt.Sprintf("%s-%s", clusterName, suffix)

	return bucketName, nil
}

// createObjectBucket creates an Object Storage bucket based on the given cluster name. This
// function will repeatedly attempt to create a bucket for the given user if one does not
// exist, sleeping for 1 second between attempts. After 10 attempts, this function will fail.
func createObjectBucket(accessKey, secretKey, endpoint, clusterName string) (string, error) {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")

	svc := s3.New(objSession, &aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: creds,
	})

	const (
		maxAttempts int           = 10
		delay       time.Duration = 100 * time.Millisecond
	)

	// Attempt to create a unique bucket. If we get an error back, inspect it to see if the
	// error indicates the bucket already exists and/or is owned by us. If it is, continue.
	// Otherwise, return the error.
	for attempt := 0; attempt < maxAttempts; attempt++ {
		bucketName, errBucketName := generateObjectBucketName(clusterName)
		if errBucketName != nil {
			return "", errBucketName
		}

		bucketConfig := s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		}

		_, errCreateBucket := svc.CreateBucket(&bucketConfig)
		if errCreateBucket == nil {
			return bucketName, nil
		}

		errAWS, ok := errCreateBucket.(awserr.Error)
		if !ok {
			return "", errCreateBucket
		}

		errorCode := errAWS.Code()
		if errorCode != s3.ErrCodeBucketAlreadyExists &&
			errorCode != s3.ErrCodeBucketAlreadyOwnedByYou {
			return "", errCreateBucket
		}

		time.Sleep(delay)
	}

	return "", fmt.Errorf("failed to create a bucket after %d attempts", maxAttempts)
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
		glog.Infof("stdout:\n%v", outStr)
	}
	if errStr != "" {
		glog.Infof("stderr:\n%v", errStr)
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
	if err := createOpaqueSecret(client, ns, "k8s-certs", k8sCerts, false, ""); err != nil {
		return err
	}

	// Write secrets for the etcd PKI material
	if err := createOpaqueSecret(client, ns, "etcd-certs", etcdCerts, false, ""); err != nil {
		return err
	}

	// Write secrets for each of the client kubeconfigs that we generated for
	// the admin, controller-manager, scheduler, and kubelet
	for secretName, secretMap := range kubeconfigs {
		if err := createOpaqueSecret(client, ns, secretName, secretMap, false, ""); err != nil {
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
	return createOpaqueSecret(client, cluster.GetNamespace(), name, data, false, "")
}

/*
 * create a secret containing artifactory credentials
 *
 *   apiVersion: v1
 *   kind: Secret
 *   type: kubernetes.io/dockerconfigjson
 *   metadata:
 *   annotations:
 *     name: artifactory-creds
 *   namespace: kube-system-$CLUSTER_NAME
 *   stringData:
 *     .dockerconfigjson: '{"auths":{"linode-docker.artifactory.linode.com":{"username":"lke-reader","password":"<REDACTED>","auth":"<REDACTED>"}}}'
 *
 *  We currently copy the secret from the kube-system namespace.
 */
func writeContainerRegistrySecrets(client client.Client, cluster *clusterv1.Cluster) error {

	name := "artifactory-creds"

	containerRegistrySecret := &corev1.Secret{}
	if err := client.Get(context.Background(),
		types.NamespacedName{Namespace: "kube-system", Name: name},
		containerRegistrySecret); err != nil {
		return err
	}

	return createDockerSecret(client, cluster.GetNamespace(), name, containerRegistrySecret.Data, true, "")
}

/*
 * Place the internal Linode CA into a configmap in the child cluster so that the CCM and CSI can load it
 */
func writeLinodeCASecrets(client client.Client, cluster *clusterv1.Cluster) error {

	name := "linode-ca"

	linodeCAConfigMap := &corev1.ConfigMap{}
	if err := client.Get(context.Background(),
		types.NamespacedName{Namespace: "kube-system", Name: name},
		linodeCAConfigMap); err != nil {
		return err
	}

	data := map[string][]byte{"cacert.pem": []byte(linodeCAConfigMap.Data["cacert.pem"])}
	return createOpaqueSecret(client, cluster.GetNamespace(), name, data, false, ClusterFinalizer)
}

// updateObjectStorageSecret updates the child cluster's object-storage secret to contain a bucket
// name corresponding to a bucket to store etcd backups in.
func updateObjectStorageSecret(client client.Client, cluster *clusterv1.Cluster) error {
	name := "object-storage"
	namespace := cluster.GetNamespace()

	objectStorageSecret := &corev1.Secret{}
	errGet := client.Get(
		context.Background(),
		types.NamespacedName{Namespace: namespace, Name: name},
		objectStorageSecret,
	)

	if errGet != nil {
		return errGet
	}

	accessKeyBytes, ok := objectStorageSecret.Data["access"]
	if !ok {
		return fmt.Errorf("access not found in object-storage secret")
	}

	secretKeyBytes, ok := objectStorageSecret.Data["secret"]
	if !ok {
		return fmt.Errorf("secret not found in object-storage secret")
	}

	endpointBytes, ok := objectStorageSecret.Data["endpoint"]
	if !ok {
		return fmt.Errorf("endpoint not found in object-storage secret")
	}

	bucketBytes, ok := objectStorageSecret.Data["bucket"]
	if ok {
		glog.Infof("[%s] bucket %s already exists for object-storage secret, not updating", namespace, string(bucketBytes))
		return nil
	}

	bucketName, errCreateBucket := createObjectBucket(
		string(accessKeyBytes),
		string(secretKeyBytes),
		string(endpointBytes),
		cluster.Name,
	)

	if errCreateBucket != nil {
		return errCreateBucket
	}

	objectStorageSecret.Data["bucket"] = []byte(bucketName)

	return createOpaqueSecret(client, cluster.GetNamespace(), name, objectStorageSecret.Data, true, "")
}

/*
 * Update the 'linode' secret with the current environment's Linode API URL
 */
func updateLinodeSecrets(client client.Client, clusterNamespace string) error {

	name := "linode"

	linodeSecret := &corev1.Secret{}
	if err := client.Get(context.Background(),
		types.NamespacedName{Namespace: clusterNamespace, Name: name},
		linodeSecret); err != nil {
		return err
	}

	// Add the current environment's Linode API URL to the secret data
	ourLinodeURL, set := os.LookupEnv("LINODE_URL")
	if !set {
		return fmt.Errorf("[%s] LINODE_URL has not been set in the environment", clusterNamespace)
	}
	linodeSecret.Data["apiurl"] = []byte(ourLinodeURL)
	return client.Update(context.Background(), linodeSecret)
}

/*
 * create secrets needed for operation of control plane components
 */
func (lcc *LinodeClusterClient) generateSecrets(cluster *clusterv1.Cluster) error {
	glog.Infof("Creating secrets for cluster %v.", cluster.Name)
	clusterNamespace := cluster.GetNamespace()

	if err := generateCertSecrets(lcc.client, cluster); err != nil {
		glog.Errorf("[%s] Error generating certs for cluster: %v", clusterNamespace, err)
		return err
	}

	if err := generateNodeWatcherSecrets(lcc.client, cluster); err != nil {
		glog.Errorf("[%s] Error generating NodeWatcher token for cluster: %v", clusterNamespace, err)
		return err
	}

	if err := writeContainerRegistrySecrets(lcc.client, cluster); err != nil {
		glog.Errorf("[%s] Error writing ContainerRegistry secrets for cluster: %v", clusterNamespace, err)
		return err
	}

	if err := updateObjectStorageSecret(lcc.client, cluster); err != nil {
		glog.Errorf("[%s] Error updating Object Storage secret for cluster: %v", clusterNamespace, err)
	}

	if err := updateLinodeSecrets(lcc.client, clusterNamespace); err != nil {
		glog.Errorf("[%s] Error updating Linode secrets: %v", clusterNamespace, err)
		return err
	}

	if err := writeLinodeCASecrets(lcc.client, cluster); err != nil {
		glog.Errorf("[%s] Error generating Linode CA secrets: %v", clusterNamespace, err)
		return err
	}

	return nil
}
