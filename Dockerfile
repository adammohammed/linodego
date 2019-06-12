# Build the manager binary
FROM golang:1.11.5 as builder

# Copy in the go src
WORKDIR /go/src/bits.linode.com/LinodeAPI/cluster-api-provider-lke
COPY ./go.mod  .
COPY ./go.sum  .
COPY cmd/    cmd/
COPY pkg/    pkg/
COPY charts/ charts/
COPY vendor/ vendor/

# Build
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager bits.linode.com/LinodeAPI/cluster-api-provider-lke/cmd/manager

# kubeadm (for pre-generating cluster tokens)
FROM ubuntu:latest as kubeadm
RUN apt-get update
RUN apt-get install -y curl
RUN curl -sSL https://dl.k8s.io/release/v1.11.3/bin/linux/amd64/kubeadm > /usr/bin/kubeadm
RUN chmod a+rx /usr/bin/kubeadm

# wireguard for key generation
RUN apt-get install add-apt-repository && add-apt-repository -y ppa:wireguard/wireguard && apt-get install -y wireguard

# Copy the controller-manager into a thin image
FROM ubuntu:latest
WORKDIR /root/
COPY --from=builder /go/src/bits.linode.com/LinodeAPI/cluster-api-provider-lke/manager .
ENTRYPOINT ["./manager"]
