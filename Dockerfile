# syntax=docker/dockerfile:experimental

# dependencies: Get dependencies here.
FROM golang:1.12-alpine3.10 AS dependencies

WORKDIR /build

# Get packages we need in order to build the manager
RUN set -ex && \
    apk update && \
    apk upgrade && \
    apk add bash=5.0.0-r0 \
            gcc=8.3.0-r0 \
            git=2.22.0-r0 \
            libc-dev=0.7.1-r0 \
            openssh=8.0_p1-r0

# Copy the internal CA
COPY cacert.pem /usr/local/share/ca-certificates/

# Get Go toolchain stuff we don't already have
RUN go get -u golang.org/x/lint/golint

# Get kubeadm
RUN set -ex && \
    mkdir /build/bin && \
    wget -O /build/bin/kubeadm-v1.13.8 https://dl.k8s.io/release/v1.13.8/bin/linux/amd64/kubeadm && \
    chmod a+rx /build/bin/kubeadm-v1.13.8 && \
    wget -O /build/bin/kubeadm-v1.14.5 https://dl.k8s.io/release/v1.14.5/bin/linux/amd64/kubeadm && \
    chmod a+rx /build/bin/kubeadm-v1.14.5 && \
    cp /build/bin/kubeadm-v1.14.5 /build/bin/kubeadm

# Copy and run the scripts to install kubebuilder and kustomize
COPY scripts /build/scripts/

RUN set -ex && \
    sh /build/scripts/install-kubebuilder.sh && \
    sh /build/scripts/install-kustomize.sh

# Copy the go.mod and go.sum files, then get necessary dependencies
# NOTE: You will need to build this image with the --ssh=default option for this step to work.
COPY ["go.mod", "go.sum", "/build/"]

RUN --mount=type=ssh \
    set -ex && \
    update-ca-certificates && \
    mkdir ~/.ssh && \
    ssh-keyscan bits.linode.com >> ~/.ssh/known_hosts && \
    git config --global url."git@bits.linode.com:".insteadOf "https://bits.linode.com/" && \
    go mod download

# source: Source code.
FROM dependencies AS source

COPY pkg /build/pkg/
COPY cmd /build/cmd/
COPY config /build/config/

# builder: Build the manager binary.
FROM source AS builder
ARG CAPLKE_REVISION

# Make sure CAPLKE_REVISION is set, then build the manager binary
RUN set -ex && \
    (test -n "${CAPLKE_REVISION+x}" || (echo "CAPLKE_REVISION not set"; exit 1)) && \
    go build -ldflags="-X main.caplkeVersion=$CAPLKE_REVISION" -o /build/bin/manager ./cmd/manager

# runner-base: Copy build artifacts, copy kubeadm, and install runtime dependencies
FROM alpine:3.10 AS runner-base

WORKDIR /root

# Get packages we need in order to run the manager
RUN set -ex && \
    apk update && \
    apk upgrade && \
    apk add iproute2=4.20.0-r1 \
            openresolv=3.9.0-r0 \
            procps=3.3.15-r0 \
            wireguard-tools=0.0.20190601-r1

COPY --from=dependencies /build/bin/kubeadm* /usr/bin/
COPY charts/ charts/

# runner: Run the manager from the built artifact.
FROM runner-base AS runner
# Copy Helm charts, kubeadm, and the manager from their respective sources

COPY --from=builder /build/bin/manager ./manager

ENTRYPOINT ["./manager"]

# runner-dev: Run the manager from a locally built artifact.
# NOTE: This assumes the existence of a manager binary at ./build/manager. If one does not exist,
#       or the binary is not compatible with Alpine, this will fail.
FROM runner-base AS runner-dev

COPY cmd/manager ./manager

ENTRYPOINT ["./manager"]
