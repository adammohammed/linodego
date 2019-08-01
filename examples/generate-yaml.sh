#!/bin/bash

set -o errexit
set -o pipefail

die() { echo "$*" 1>&2 ; exit 1; }

[ "$#" -eq 5 ] || die 'First argument must be a path to an ssh public key (for accessing Nodes of the cluster).
Second argument must be a name for the cluster.
Third argument must be an S3 style AWS_ACCESS_KEY_ID
Fourth argument must be an S3 style AWS_SECRET_ACCESS_KEY
Fifth argument must be an S3-style AWS endpoint, without bucket
Sixth argument must be a machine name
Seventh argument must be a pool ID

For example:
./generate-yaml.sh $HOME/.ssh/id_rsa.pub cluster01 $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY $AWS_ENDPOINT $MACHINE_NAME $POOL_ID'

[ -z "${LINODE_TOKEN}" ] && die "\$LINODE_TOKEN must be set to a Linode API token"
[ -z "${LINODE_REGION}" ] && die "\$LINODE_REGION must be set to a Linode Region ID"

PUBLIC_KEY=$(cat $1)
CLUSTER_NAME=$2
ENCODED_TOKEN=$(echo -n $LINODE_TOKEN | base64)
ENCODED_REGION=$(echo -n $LINODE_REGION | base64)
ENCODED_ACCESS_KEY=$(echo -n $3 | base64)
ENCODED_SECRET_KEY=$(echo -n $4 | base64)
ENCODED_AWS_ENDPOINT=$(echo -n $5 | base64)
MACHINE_NAME="$6"
POOL_ID="$7"

cat cluster.yaml.template |
sed -e "s|\$SSH_PUBLIC_KEY|$(cat $1)|" |
sed -e "s|\$LINODE_TOKEN|$ENCODED_TOKEN|" |
sed -e "s|\$LINODE_REGION|$ENCODED_REGION|" |
sed -e "s|\$AWS_ACCESS_KEY_ID|$ENCODED_ACCESS_KEY|" |
sed -e "s|\$AWS_SECRET_ACCESS_KEY|$ENCODED_SECRET_KEY|" |
sed -e "s|\$AWS_ENDPOINT|$ENCODED_AWS_ENDPOINT|" |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > cluster.yaml

cat master.yaml.template |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > master.yaml

cat nodes.yaml.template |
sed -e "s|\$MACHINE_NAME|$MACHINE_NAME|" |
sed -e "s|\$POOL_ID|$POOL_ID|" |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > nodes.yaml
