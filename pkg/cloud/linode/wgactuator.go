/*
Copyright 2019 Linode, LLC.
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

	"bits.linode.com/aprotopopov/wg-node-controller/helpers"
)

func generateWGKeys() (string, string, error) {
	privfile, err := tempfile("", []byte{})
	if err != nil {
		return "", "", fmt.Errorf("can't create a temporary file: %v", err)
	}
	defer os.Remove(privfile)

	wgpub, err := run("bash", "-c", fmt.Sprintf("wg genkey | tee %s | wg pubkey", privfile))
	if err != nil {
		return "", "", fmt.Errorf("can't generate WG keys: %v", err)
	}

	wgpriv, err := run("cat", privfile)
	if err != nil {
		return "", "", fmt.Errorf("can't read a file: %v", err)
	}

	return wgpub, wgpriv, nil
}

func (lc *LinodeClient) savePrivateKey(key, namespace string) (string, error) {
	name := "wgpriv"

	data := map[string][]byte{
		"key": []byte(key),
	}

	err := createOpaqueSecret(lc.client, namespace, name, data)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (lc *LinodeClient) getWGwgPubKey(cluster string) (string, error) {
	namespace := clusterNamespace(cluster)

	config, err := helpers.GetAPIConfig(lc.clusterConfigClient, namespace)
	if err != nil {
		wgpub, wgpriv, err := generateWGKeys()
		if err != nil {
			return "", fmt.Errorf("Failed to generate WG keys: %v", err)
		}

		wgprivname, err := lc.savePrivateKey(wgpriv, namespace)
		if err != nil {
			return "", fmt.Errorf("Failed to save WG private key in a secret: %v", err)
		}

		config, err = helpers.CreateAPIConfig(lc.clusterConfigClient, namespace, wgpub, wgprivname)
		if err != nil {
			return "", fmt.Errorf("Couldn't init initial WG config: %v", err)
		}
	}

	if L := len(config.APIServers); L != 1 {
		return "", fmt.Errorf("Corrupted WG config: len(config.APIServers)=%d", L)
	}
	return config.APIServers[0].WGPublicKey, nil
}
