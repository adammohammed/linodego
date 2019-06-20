# Image URL to use all building/pushing image targets
IMG ?= linode-docker.artifactory.linode.com/asauber/cluster-api-provider-lke:canaryrc1
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
export GO111MODULE=on

# define phony targets which do not actually map to filenames
.PHONY: all generate vet fmt test manager run run-docker installcrds deploy manifests docker-build docker-push

.PHONY: all
all: test manager

# resolve and update dependencies
.PHONY: deps
deps:
	go mod tidy

# Generate code using go code generation.
# Used to generate DeepCopy functions for Kubernetes Resource defintions
generate:
	go generate ./pkg/... ./cmd/...

# Run go vet against all files.
# Go vet is a linter. Please fix all issues that it finds.
vet: generate
	go vet ./pkg/... ./cmd/...

# Run go fmt against all files.
fmt: vet
	go fmt ./pkg/... ./cmd/...

# Generate manifests e.g. CRD, RBAC etc.
manifests: fmt
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd --output-dir config/default/crds
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac --output-dir config/default/rbac

# Run tests
test: fmt manifests
	go test -v ./pkg/... ./cmd/... -coverprofile cover.out

# Build binary
manager: test
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ./run/manager ./cmd/manager

# Build binary
build: manager
	echo "Building cluster-api-provider-lke controller manager binary"

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt
	go run ./cmd/manager/main.go -logtostderr=true -stderrthreshold=INFO

# Run in Linux container against the configured Kubernetes cluster in the file at $KUBECONFIG
# Do not push and run this image from Kubernetes by image name, it will run out of threads while compiling :-)
run-docker: fmt
	@mkdir -p ${ROOT_DIR}/run
	docker build -t "cluster-api-provider-lke:devel-run" -f Dockerfile.devel .
	echo "Running the controller.. ctrl-c to stop, ctrl-z to detach (then use docker ps, docker attach, docker kill)"
	go mod vendor
	docker run -e KUBECONFIG=/root/.kube/config \
		--detach-keys "ctrl-z" \
		-e RUNNING_EXTERNALLY=yes \
		-v $${KUBECONFIG}:/root/.kube/config \
		-v ${ROOT_DIR}/run:/tmp/ \
		"-ti" \
		"cluster-api-provider-lke:devel-run" -logtostderr=true -stderrthreshold=INFO

# Install CRDs into a cluster
installcrds: manifests
	kubectl apply -f config/default/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kustomize build config/default | kubectl apply -f -

# Build the docker image
docker-build: build
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i.bak -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

