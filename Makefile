# Image URL to use all building/pushing image targets, can be overridden by environment
IMG ?= cluster-api-provider-lke:canary
SRC_IMG ?= cluster-api-provider-lke:canary-source

# Linode API URL to be copied to a child cluster secret, can be overrideen by environment
LINODE_URL ?= https://api.dev.linode.com/v4
# CAPLKE version (defaults to output of revision.sh)
CAPLKE_REVISION ?= $(shell sh revision.sh)

ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Base files for generated source - these will need to be updated if manifests or the source
# go generate depends on are changed
GO_GENERATE_SRC = $(shell find "pkg/apis/lkeproviderconfig/v1alpha1" -name "*.go" | grep -v "generated")
KUSTOMIZE_MANIFESTS = $(wildcard config/default/*.yaml)

# Generated source paths - these will need to be updated if there are any changes to generated files
GENERATED_SRC = pkg/apis/lkeproviderconfig/v1alpha1/zz_generated.deepcopy.go
CRD_MANIFESTS := config/default/crds/lkeproviderconfig_v1alpha1_lkemachineproviderconfig.yaml config/default/crds/lkeproviderconfig_v1alpha1_lkeclusterproviderconfig.yaml
RBAC_MANIFESTS := config/default/rbac/rbac_role.yaml config/default/rbac/rbac_role_binding.yaml

# Lock file templates for CRD and RBAC manifest generation
CRD_LOCK := $(shell mktemp -u /tmp/crd.lockXXX)
RBAC_LOCK := $(shell mktemp -u /tmp/rbac.lockXXX)

export GO111MODULE=on

.PHONY: all
all: test manager

# resolve and update dependencies
.PHONY: deps
deps:
	go mod tidy

# Generate code using go code generation.
# Used to generate DeepCopy functions for Kubernetes Resource defintions
$(GENERATED_SRC): $(GO_GENERATE_SRC)
	@if [ -z "${GOPATH}" ]; then echo 'You must have your local $$GOPATH set' && exit 1; fi
	@# The follow directory appears erronerously if the local $$GOPATH was not set
	@rm -rf ./pkg/apis/bits.linode.com
	go generate ./pkg/... ./cmd/...

.PHONY: generate
generate: $(GENERATED_SRC)

$(CRD_MANIFESTS):
	(set -C && echo > $(CRD_LOCK)) 2>/dev/null; \
	if [ $$? -eq 0 ]; then \
		go run sigs.k8s.io/controller-tools/cmd/controller-gen crd --output-dir config/default/crds; \
		rm $(CRD_LOCK); \
	fi

$(RBAC_MANIFESTS):
	(set -C && echo > $(RBAC_LOCK)) 2>/dev/null; \
	if [ $$? -eq 0 ]; then \
		go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac --output-dir config/default/rbac; \
		rm $(RBAC_LOCK); \
	fi

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: $(CRD_MANIFESTS) $(RBAC_MANIFESTS)

.PHONY: docker-src
docker-src:
	DOCKER_BUILDKIT=1 docker build --quiet --ssh=default --progress=plain -t=${SRC_IMG} --target=source .

# Run go vet against all files.
# Go vet is a linter. Please fix all issues that it finds.
vet: generate docker-src
	docker run --rm ${SRC_IMG} go vet ./pkg/... ./cmd/...

# Run go fmt against all files.
.PHONY: fmt
fmt: generate
	go fmt ./pkg/... ./cmd/...

# Run the go linter against all files
.PHONY: lint
lint: generate docker-src
	docker run --rm ${SRC_IMG} golint ./pkg/... ./cmd/...

# Run tests
.PHONY: test
test: generate manifests docker-src
	touch cover.out && \
	docker run --rm --mount type=bind,src=$(shell pwd)/cover.out,target=/build/cover.out \
		${SRC_IMG} go test -v ./pkg/... ./cmd/... -coverprofile /build/cover.out

# Build binary
.PHONY: manager
manager: generate manifests
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.caplkeVersion=$(CAPLKE_REVISION)" -o ./run/manager ./cmd/manager

# Build binary
.PHONY: build
build: manager
	echo "Building cluster-api-provider-lke controller manager binary"

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: fmt
	go run -ldflags "-X main.caplkeVersion=$(CAPLKE_REVISION)" ./cmd/manager/main.go -logtostderr=true -stderrthreshold=INFO

# Run in Linux container against the configured Kubernetes cluster in the file at $KUBECONFIG
# Do not push and run this image from Kubernetes by image name, it will run out of threads while compiling :-)
.PHONY: run-docker
run-docker: build
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
docker-build:
	DOCKER_BUILDKIT=1 docker build --ssh=default --progress=plain -t=${IMG} --target=runner --build-arg=CAPLKE_REVISION=$(CAPLKE_REVISION) .
	@echo "updating kustomize image patch file for manager resource"
	sed -i.bak -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Build a developer version of the Docker image
.PHONY: docker-build-dev
docker-build-dev: manager
	DOCKER_BUILDKIT=1 docker build --ssh=default --progress=plain -t=${IMG} --target=runner-dev --build-arg=CAPLKE_REVISION=$(CAPLKE_REVISION) .
	@echo "updating kustomize image patch file for manager resource"
	sed -i.bak -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

