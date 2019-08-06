#!/bin/bash

set -o errexit
set -o pipefail

die() { echo "$*" 1>&2 ; exit 1; }

USAGE="Usage: generate-yaml.sh -t linode-token -r linode-region -k pubkey -c cluster-name -a obj-access-key -s obj-secret-key -e obj-endpoint -b obj-bucket -m machine-name -p pool-id"

while (( "$#" )); do
    case "$1" in
        -t|--linode-token)
            LINODE_TOKEN=$2
            shift 2
            ;;
        -r|--linode-region)
            LINODE_REGION=$2
            shift 2
            ;;
	-k|--public-key)
            PUBLIC_KEY_PATH=$2
            shift 2
            ;;
        -c|--cluster-name)
            CLUSTER_NAME=$2
            shift 2
            ;;
        -a|--obj-access-key)
            OBJ_ACCESS_KEY=$2
            shift 2
            ;;
        -s|--obj-secret-key)
            OBJ_SECRET_KEY=$2
            shift 2
            ;;
        -e|--obj-endpoint)
            OBJ_ENDPOINT=$2
            shift 2
            ;;
        -b|--obj-bucket)
            OBJ_BUCKET=$2
            shift 2
            ;;
        -m|--machine-name)
            MACHINE_NAME=$2
            shift 2
            ;;
        -p|--pool-id)
            POOL_ID=$2
            shift 2
            ;;
        -h|--help)
            die "$USAGE"
            ;;
        *)
            echo "unknown argument $1" 1>&2
            die "$USAGE"
            ;;
    esac
done

[ -z "${LINODE_TOKEN}" ] && die "LINODE_TOKEN must be set to a Linode API token"
[ -z "${LINODE_REGION}" ] && die "LINODE_REGION must be set to a Linode Region ID"

PUBLIC_KEY=$(cat $PUBLIC_KEY_PATH)
ENCODED_TOKEN=$(echo -n $LINODE_TOKEN | base64)
ENCODED_REGION=$(echo -n $LINODE_REGION | base64)
ENCODED_ACCESS_KEY=$(echo -n $OBJ_ACCESS_KEY | base64)
ENCODED_SECRET_KEY=$(echo -n $OBJ_SECRET_KEY | base64)
ENCODED_AWS_ENDPOINT=$(echo -n $OBJ_ENDPOINT | base64)
ENCODED_AWS_BUCKET=$(echo -n $OBJ_BUCKET | base64)

cat cluster.yaml.template |
sed -e "s|\$SSH_PUBLIC_KEY|$PUBLIC_KEY|" |
sed -e "s|\$LINODE_TOKEN|$ENCODED_TOKEN|" |
sed -e "s|\$LINODE_REGION|$ENCODED_REGION|" |
sed -e "s|\$AWS_ACCESS_KEY_ID|$ENCODED_ACCESS_KEY|" |
sed -e "s|\$AWS_SECRET_ACCESS_KEY|$ENCODED_SECRET_KEY|" |
sed -e "s|\$AWS_BUCKET|$ENCODED_AWS_BUCKET|" |
sed -e "s|\$AWS_ENDPOINT|$ENCODED_AWS_ENDPOINT|" |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > cluster.yaml

cat master.yaml.template |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > master.yaml

cat nodes.yaml.template |
sed -e "s|\$MACHINE_NAME|$MACHINE_NAME|" |
sed -e "s|\$POOL_ID|$POOL_ID|" |
sed -e "s|\$CLUSTER_NAME|$CLUSTER_NAME|" > nodes.yaml
