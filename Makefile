# Image URL to use all building/pushing image targets, can be overridden by environment
IMG ?= linode-docker.artifactory.linode.com/lke/cluster-api-provider-lke:canary
# Linode API URL to be copied to a child cluster secret, can be overrideen by environment
LINODE_URL ?= https://api.dev.linode.com/v4

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
REVISION = $(shell sh revision.sh)
export GO111MODULE=on

.PHONY: all
all: test manager

# resolve and update dependencies
.PHONY: deps
deps:
	go mod tidy

# Generate code using go code generation.
# Used to generate DeepCopy functions for Kubernetes Resource defintions
.PHONY: generate
generate:
	go generate ./pkg/... ./cmd/...

# Run go vet against all files.
# Go vet is a linter. Please fix all issues that it finds.
.PHONY: vet
vet: generate
	go vet ./pkg/... ./cmd/...

# Run go fmt against all files.
.PHONY: fmt
fmt: vet
	go fmt ./pkg/... ./cmd/...

# Run the go linter against all files
.PHONY: lint
lint: fmt
	golint ./pkg/... ./cmd/...


# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: fmt
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd --output-dir config/default/crds
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac --output-dir config/default/rbac

# Run tests
.PHONY: test
test: fmt manifests
	go test -v ./pkg/... ./cmd/... -coverprofile cover.out

# Build binary
.PHONY: manager
manager: test
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.caplkeVersion=$(REVISION)" -a -o ./run/manager ./cmd/manager

# Build binary
.PHONY: build
build: manager
	echo "Building cluster-api-provider-lke controller manager binary"

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: fmt
	go run -ldflags "-X main.caplkeVersion=$(REVISION)" ./cmd/manager/main.go -logtostderr=true -stderrthreshold=INFO

# Run in Linux container against the configured Kubernetes cluster in the file at $KUBECONFIG
# Do not push and run this image from Kubernetes by image name, it will run out of threads while compiling :-)
.PHONY: run-docker
run-docker: fmt
	@mkdir -p ${ROOT_DIR}/run
	docker build -t "cluster-api-provider-lke:devel-run" -f Dockerfile.devel .
	echo "Running the controller.. ctrl-c to stop, ctrl-z to detach (then use docker ps, docker attach, docker kill)"
	go mod vendor
	docker run -e KUBECONFIG=/root/.kube/config \
		--detach-keys "ctrl-z" \
		-e RUNNING_EXTERNALLY=yes \
		-e LINODE_CA=/cacert.pem \
		-e LINODE_URL=${LINODE_URL} \
		-v $${KUBECONFIG}:/root/.kube/config \
		-v ${ROOT_DIR}/run:/tmp/ \
		"-ti" \
		"cluster-api-provider-lke:devel-run" -logtostderr=true -stderrthreshold=INFO

# Install CRDs into a cluster
.PHONY: installcrds
installcrds: manifests
	kubectl apply -f config/default/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests
	kustomize build config/default | kubectl apply -f -

.PHONY: imgname
# print the Docker image name that will be used
# useful for subsequently defining it on the shell
imgname:
	echo "export IMG=${IMG}"

# Build the docker image
.PHONY: docker-build
docker-build: build
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i.bak -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

