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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// run_prog executes a local process prog and returns the standard output of that
// process and an error, if any.
func run_prog(prog string, args ...string) (string, error) {
	glog.Infof("running cmd='%s' args=%v", prog, args)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(prog, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if outStr != "" {
		glog.Infof("%s: STDOUT='%s'", prog, strings.TrimSpace(outStr))
	}
	if errStr != "" {
		glog.Infof("%s: STDERR='%s'", prog, strings.TrimSpace(errStr))
	}

	return strings.TrimSpace(outStr), err
}

// system runs a command like system(3), but also accepts formatting arguments
func system(cmd_format string, args ...interface{}) (string, error) {
	return run_prog("bash", "-c", fmt.Sprintf(cmd_format, args...))
}

/*
 * getJoinToken returns a valid bootstrap token for a LKE cluster specified in
 * the command line arguments. If token doesn't exist, the function creates it.
 * The function also tries to remove all expired tokens from the cluster.
 */
func getJoinToken(cpcClient client.Client, cluster string) (string, error) {

	/*
	 * A temporary kube config, as kubeadm requires a file argument
	 */
	kubeconfig, err := tempKubeconfig(cpcClient, cluster)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfig)

	/*
	 * Delete all tokens which had expired since we are here.
	 * Such tokens are marked as <invalid> by kubeadm
	 */
	if _, err := system("kubeadm --kubeconfig %[1]s token list | awk '$2 == \"<invalid>\" { system(\"kubeadm --kubeconfig %[1]s token delete \" $1) }'", kubeconfig); err != nil {
		return "", err
	}

	/* get the first non-expired token */
	token, err := system("kubeadm --kubeconfig %s token list | awk 'NR>1 && !($2==\"<invalid>\") {print $1; exit}'", kubeconfig)
	if err != nil {
		return "", err
	} else if token != "" {
		return token, nil
	}

	/* didn't find a token, try to create it */
	return system("kubeadm --kubeconfig %s token create --ttl 1h", kubeconfig)
}
