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
	"strconv"
	"strings"

	lkeconfigv1 "bits.linode.com/LinodeAPI/cluster-api-provider-lke/pkg/apis/lkeproviderconfig/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/linode/linodego"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	bootstraputil "k8s.io/client-go/tools/bootstrap/token/util"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/kubeadm"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	lkeclient "bits.linode.com/LinodeAPI/wg-controller/pkg/client"
)

const (
	ProviderName = "lke"
)

const (
	createEventAction             = "Create"
	deleteEventAction             = "Delete"
	noEventAction                 = ""
	linodeAPITokenSecretName      = "linode"
	machineLinodeIDAnnotationName = "linode-id"
)

type LinodeClient struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
	kubeadm       *kubeadm.Kubeadm

	clusterConfigClient *lkeclient.ConfigV1Alpha1Client
}

type MachineActuatorParams struct {
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

func NewMachineActuator(m manager.Manager, params MachineActuatorParams) (*LinodeClient, error) {
	clusterConfigClient, err := newClusterConfigClient()
	if err != nil {
		return nil, err
	}

	return &LinodeClient{
		client:              m.GetClient(),
		scheme:              params.Scheme,
		eventRecorder:       params.EventRecorder,
		kubeadm:             kubeadm.New(),
		clusterConfigClient: clusterConfigClient,
	}, nil
}

func newClusterConfigClient() (*lkeclient.ConfigV1Alpha1Client, error) {
	/*
	 * If we are running externally to cluster (in a docker container),
	 * then the kubeconfig file is "/root/.kube/config". Otherwise we can
	 * create a client from the in-cluster config
	 */
	if os.Getenv("RUNNING_EXTERNALLY") == "yes" {
		return lkeclient.NewClientFromFlags("/root/.kube/config")
	} else {
		return lkeclient.NewInClusterClient()
	}
}

func getLinodeAPIClient(client client.Client, cluster *clusterv1.Cluster) (*linodego.Client, string, error) {
	/*
	 * We construct a new client every time that we make a Linode API call so that
	 * the API Token Secret can be rotated at any time. We need a Cluster object
	 * so that we can associate a different API token with each Cluster.
	 */
	apiTokenSecret := &corev1.Secret{}
	namespace := cluster.GetNamespace()
	err := client.Get(context.Background(),
		types.NamespacedName{Namespace: namespace, Name: linodeAPITokenSecretName},
		apiTokenSecret)

	if err != nil {
		return nil, "", fmt.Errorf("error retrieving Linode API token secret for cluster %v", err)
	}

	apiKey, ok := apiTokenSecret.Data["token"]
	if !ok {
		return nil, "", fmt.Errorf("Linode API token secret for namespace %s is missing 'token' data", namespace)
	}

	region, ok := apiTokenSecret.Data["region"]
	if !ok {
		return nil, "", fmt.Errorf("Linode API token secret for namespace %s is missing 'region' data", namespace)
	}

	linodeClient := linodego.NewClient(nil)
	linodeClient.SetUserAgent(fmt.Sprintf("cluster-api-provider-lke %s", linodego.DefaultUserAgent))
	linodeClient.SetToken(string(apiKey))
	if len(os.Getenv("LINODEGO_DEBUG")) > 0 {
		linodeClient.SetDebug(true)
	}
	return &linodeClient, strings.TrimSpace(string(region)), nil
}

func (lc *LinodeClient) validateMachine(machine *clusterv1.Machine, config *lkeconfigv1.LkeMachineProviderConfig) *apierrors.MachineError {
	if machine.Spec.Versions.Kubelet == "" {
		return apierrors.InvalidMachineConfiguration("spec.versions.kubelet can't be empty")
	}
	return nil
}

func (lc *LinodeClient) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {

	err := lc.create(ctx, cluster, machine)
	klog.Infof("creating machine %v/%v: err=%v", cluster.Name, machine.Name, err)
	return err

}

func (lc *LinodeClient) create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v/%v", cluster.Name, machine.Name)

	instance, err := lc.instanceIfExists(cluster, machine)
	if err != nil {
		return err
	}

	if instance == nil {
		machineConfig, err := machineProviderConfig(machine.Spec.ProviderSpec)
		if err != nil {
			return lc.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
				"Cannot unmarshal machine's providerConfig field: %v", err), createEventAction)
		}
		if verr := lc.validateMachine(machine, machineConfig); verr != nil {
			return lc.handleMachineError(machine, verr, createEventAction)
		}

		clusterConfig, err := clusterProviderConfig(cluster.Spec.ProviderSpec)
		if clusterConfig == nil {
			return fmt.Errorf("LKE cluster spec was not provided for cluster %v", cluster.Name)
		}
		if err != nil {
			return err
		}

		token, err := getJoinToken(lc.client, cluster.Name)
		if err != nil {
			return err
		}

		wgPubKey, err := lc.getWGwgPubKey(cluster.Name)
		if err != nil {
			return fmt.Errorf("Couldn't get WG public key for cluster %s: %v", cluster.Name, err)
		}

		klog.Infof("roles %v", machineConfig.Roles)
		initScript, err := lc.getInitScript(token, cluster, machine, machineConfig, wgPubKey)
		if err != nil {
			return err
		}

		linodeClient, CPCRegion, err := getLinodeAPIClient(lc.client, cluster)
		if err != nil {
			return fmt.Errorf("Error initializing Linode API client: %v", err)
		}

		/*
		* Use a bootstrap token as a random root password. Replace this if the
		* function is removed upstream. Don't store this - the idea is that no one
		* ever knows the root password.
		 */
		rootPass, err := bootstraputil.GenerateBootstrapToken()
		if err != nil {
			return fmt.Errorf("Couldn't generate random root password: %v", err)
		}

		instance, err := linodeClient.CreateInstance(context.Background(), linodego.InstanceCreateOptions{
			Region:          CPCRegion,
			Type:            machineConfig.Type,
			Label:           lc.MachineLabel(cluster, machine),
			Image:           machineConfig.Image,
			RootPass:        rootPass,
			PrivateIP:       true,
			StackScriptID:   initScript.stackScript.ID,
			StackScriptData: initScript.stackScriptData,
			AuthorizedKeys:  clusterConfig.AuthorizedKeys,
		})
		instanceCreationTimeoutSeconds := 600
		if err == nil {
			instance, err = linodeClient.WaitForInstanceStatus(
				context.Background(), instance.ID, linodego.InstanceRunning, instanceCreationTimeoutSeconds)
		}

		if err != nil {
			return lc.handleMachineError(machine, apierrors.CreateMachine(
				"error creating Linode instance: %v", err), createEventAction)
		}

		if isMaster(machineConfig.Roles) {
			lc.updateClusterEndpoint(cluster, instance)
		}

		lc.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Created", "Created Machine %v", machine.Name)

		/* Annotate Machine object with Linode ID */
		lc.AnnotateMachine(machine, machineLinodeIDAnnotationName, strconv.FormatInt(int64(instance.ID), 10))
	} else {
		klog.Infof("Skipped creating a VM that already exists.\n")
	}
	return nil
}

func (lc *LinodeClient) AnnotateMachine(machine *clusterv1.Machine, key string, value string) error {
	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	machine.ObjectMeta.Annotations[key] = value
	klog.Infof("Annotating machine with %s: %s", key, value)
	return lc.client.Update(context.Background(), machine)
}

func (lc *LinodeClient) updateClusterEndpoint(cluster *clusterv1.Cluster, instance *linodego.Instance) error {
	/* Find the public IPv4 address for the master instance */
	/* TODO: When we support HA masters, this will be a load balancer hostname */
	for _, ip := range instance.IPv4 {
		ipString := ip.String()
		if !strings.HasPrefix(ipString, "192.168.") {
			klog.Infof("Updating cluster endpoint %v.\n", ipString)
			cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{{
				Host: ipString,
				Port: 6443,
			}}
			err := lc.client.Update(context.Background(), cluster)
			return err
		}
	}
	return fmt.Errorf("Could not determine endpoint for machine %v", instance)
}

func (lc *LinodeClient) handleMachineError(machine *clusterv1.Machine, err *apierrors.MachineError, eventAction string) error {
	/* TODO: implement machine.Status update on error */
	/*
		if lc.client != nil {
			reason := err.Reason
			message := err.Message
			machine.Status.ErrorReason = &reason
			machine.Status.ErrorMessage = &message
		}
	*/

	if eventAction != noEventAction {
		lc.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err.Reason)
	}

	klog.Errorf("Machine error: %v", err.Message)
	return err
}

func (lc *LinodeClient) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	linodeIDStr, ok := machine.ObjectMeta.Annotations[machineLinodeIDAnnotationName]
	if !ok {
		klog.Infof("Deleting machine resource, but no Linode deleted (no Linode ID annotation)")
		return nil
	}
	klog.Infof("Deleting Linode with ID %s", linodeIDStr)

	linodeID, err := strconv.Atoi(linodeIDStr)
	if err != nil {
		return fmt.Errorf("Error converting Linode ID annotation to integer")
	}

	linodeClient, _, err := getLinodeAPIClient(lc.client, cluster)
	if err != nil {
		return fmt.Errorf("Error initializing Linode API client: %v", err)
	}

	err = linodeClient.DeleteInstance(context.Background(), linodeID)
	if err != nil {

		original_err, ok := err.(*linodego.Error)

		/*
		 * If a linode with the linodeIDStr ID dosn't exist, then we
		 * assume that a user deleted the linode voluntary and thus we
		 * still need to delete the machine. Otherwise, we can get into
		 * an infinite loop and won't be able to delete a machine.
		 */
		if ok && original_err.Code == 404 {
			klog.Infof("Linode with ID %s doesn't exist; Deleting machine anyway", linodeIDStr)
			return nil
		}

		return fmt.Errorf("Error deleting Linode %d: %s", linodeID, err.Error())
	}

	klog.Infof("Deleted Linode with ID %s", linodeIDStr)
	return nil
}

func (lc *LinodeClient) Update(ctx context.Context, cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	klog.Infof("TODO (Not Implemented): Updating machine with cluster %v.", cluster.Name)
	klog.Infof("TODO (Not Implemented): Updating machine %v.", goalMachine.Name)
	return nil
}

func (lc *LinodeClient) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	klog.Infof("Checking Exists for machine %v/%v", cluster.Name, machine.Name)
	instance, err := lc.instanceIfExists(cluster, machine)
	if err != nil {
		return false, err
	}
	return (instance != nil), err
}

func clusterProviderConfig(providerConfig clusterv1.ProviderSpec) (*lkeconfigv1.LkeClusterProviderConfig, error) {
	var config lkeconfigv1.LkeClusterProviderConfig
	if err := yaml.Unmarshal(providerConfig.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (lc *LinodeClient) MachineLabel(cluster *clusterv1.Cluster, machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-%s", cluster.ObjectMeta.Name, machine.ObjectMeta.Name)
}

func machineProviderConfig(providerConfig clusterv1.ProviderSpec) (*lkeconfigv1.LkeMachineProviderConfig, error) {
	var config lkeconfigv1.LkeMachineProviderConfig
	if err := yaml.Unmarshal(providerConfig.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// Gets the instance represented by the given machine
func (lc *LinodeClient) instanceIfExists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*linodego.Instance, error) {
	identifyingMachine := machine

	// Try to use the last saved status locating the machine
	status, err := lc.instanceStatus(machine)
	if err != nil {
		return nil, err
	}

	if status != nil {
		identifyingMachine = (*clusterv1.Machine)(status)
	}

	// Get the VM via Linode label: <cluster-name>-<machine-name>
	label := lc.MachineLabel(cluster, identifyingMachine)
	linodeClient, _, err := getLinodeAPIClient(lc.client, cluster)
	if err != nil {
		return nil, fmt.Errorf("Error initializing Linode API client: %v", err)
	}
	instance, err := getInstanceByLabel(linodeClient, label)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func getInstanceByLabel(linodeClient *linodego.Client, label string) (*linodego.Instance, error) {
	filter := fmt.Sprintf("{ \"label\": \"%s\" }", label)
	instances, err := linodeClient.ListInstances(context.Background(), &linodego.ListOptions{
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}
	if len(instances) < 1 {
		return nil, nil
	}
	return &instances[0], nil
}

func (lc *LinodeClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	klog.Infof("TODO (Not Implemented): Getting IP for machine with cluster %v.", cluster.Name)
	klog.Infof("TODO (Not Implemented): Getting IP for machine %v.", machine.Name)
	return "", nil
}

func (lc *LinodeClient) GetKubeConfig(cluster *clusterv1.Cluster, master *clusterv1.Machine) (string, error) {
	klog.Infof("TODO (Not Implemented): Getting KubeConfig for master with cluster %v.", cluster.Name)
	klog.Infof("TODO (Not Implemented): Getting KubeConfig for master %v.", master.Name)
	return "", nil
}
