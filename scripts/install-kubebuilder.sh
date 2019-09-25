#!/bin/bash -e

os=$(go env GOOS)
arch=$(go env GOARCH)

if [ $os != "linux" ] || [ $arch != "amd64" ]; then
    echo "Unsupported architecture (must be linux/amd64)" && exit 1
fi

cd /tmp
wget -q https://go.kubebuilder.io/dl/2.0.0/$os/$arch -O kubebuilder.tar.gz

# Busybox doesn't support process substitution, <( ... ), to do this in the cool way
echo "858d84aa3e8bb6528d7dd4dbab4e8fceb59c8ea7905918bc72dc719d784c40f3  kubebuilder.tar.gz" > checksum_kubebuilder.txt
sha256sum -c checksum_kubebuilder.txt

tar xzf kubebuilder.tar.gz
mv kubebuilder_2.0.0_${os}_${arch} /usr/local/kubebuilder

export PATH=$PATH:/usr/local/kubebuilder/bin
