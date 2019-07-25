FROM alpine:edge
WORKDIR /root/
COPY charts/ charts/
COPY ./run/manager .
RUN \
    echo http://dl-cdn.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories && \
    apk update && \
    apk upgrade && \
    apk add bash iproute2 openresolv procps curl wireguard-tools && \
    curl -sSL https://dl.k8s.io/release/v1.13.4/bin/linux/amd64/kubeadm > /usr/bin/kubeadm && \
    chmod a+rx /usr/bin/kubeadm
COPY ./run/manager .
ENTRYPOINT ["./manager"]
