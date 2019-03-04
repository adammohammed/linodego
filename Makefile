
# Image URL to use all building/pushing image targets
IMG ?= linode-docker.artifactory.linode.com/asauber/cluster-api-provider-lke:latest
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

export GO111MODULE=on

all: test manager

# Run tests
test: generate fmt vet manifests
	go test -v ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager bits.linode.com/asauber/cluster-api-provider-lke/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go -logtostderr=true -stderrthreshold=INFO

# Run in Linux container against the configured Kubernetes cluster in the file at $KUBECONFIG
# Do not push and run this image from Kubernetes by image name, it will run out of threads while compiling :-)
run-docker: generate fmt vet
	@mkdir -p ${ROOT_DIR}/run
	kubectl apply -f ./provider-components.yaml
	docker build -t "cluster-api-provider-lke:devel-run" -f Dockerfile.devel .
	echo "Running the controller.. ctrl-c to stop, ctrl-z to detach (then use docker ps, docker attach, docker kill)"
	docker run -e KUBECONFIG=/root/.kube/config \
		--detach-keys "ctrl-z" \
		-v $${KUBECONFIG}:/root/.kube/config \
		-v ${ROOT_DIR}/run:/tmp/ \
		"-ti" \
		"cluster-api-provider-lke:devel-run" -logtostderr=true -stderrthreshold=INFO

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd --output-dir config/default/crds
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac --output-dir config/default/rbac

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i.bak -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
