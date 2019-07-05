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

package main

import (
	"flag"

	"bits.linode.com/LinodeAPI/cluster-api-provider-lke/pkg/apis"
	"bits.linode.com/LinodeAPI/cluster-api-provider-lke/pkg/cloud/linode"
	"bits.linode.com/LinodeAPI/cluster-api-provider-lke/pkg/controller"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"
	clustercommon "sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// set at link-time (see the Makefile)
var caplkeVersion string

func main() {
	klog.Infof("cluster-api provider LKE starting up with verison %s\n", caplkeVersion)

	// Init klog
	klog.InitFlags(nil)

	// parse flags from all packages
	flag.Parse()

	klog.Infof("Cluster API Provider LKE starting up.")

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		klog.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		klog.Fatal(err)
	}

	klog.Infof("Initializing Dependencies.")
	machineActuator, err := linode.NewMachineActuator(mgr, linode.MachineActuatorParams{
		Scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetRecorder("linode-controller"),
	})
	if err != nil {
		klog.Fatal(err)
	}
	clustercommon.RegisterClusterProvisioner(linode.ProviderName, machineActuator)

	klog.Infof("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		klog.Fatal(err)
	}

	klog.Infof("Starting the Cmd.")

	// Start the Cmd
	klog.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
