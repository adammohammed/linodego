#!/bin/sh -e

wget -q https://github.com/kubernetes-sigs/kustomize/releases/download/v3.1.0/kustomize_3.1.0_linux_amd64

echo "73acc575cf4e035a91da63ecffcabe58f9572562b772c1eb7ed863991950afe8  kustomize_3.1.0_linux_amd64" > checksum_kustomize.txt

sha256sum -c checksum_kustomize.txt

mv kustomize_3.1.0_linux_amd64 /usr/local/bin/kustomize
