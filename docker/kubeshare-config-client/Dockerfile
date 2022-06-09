# call from repo root

FROM ubuntu:18.04 AS build

ENV GOLANG_VERSION 1.13.5
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/github.com/Interstellarss/faas-share

COPY . .

RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64/3bf863cc.pub
RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/machine-learning/repos/ubuntu1804/x86_64/7fa2af80.pub

RUN apt update && \
    apt install -y g++ wget make && \
    wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    make kubeshare-config-client

FROM debian:stretch-slim

ENV NVIDIA_VISIBLE_DEVICES      all
ENV NVIDIA_DRIVER_CAPABILITIES  utility

COPY --from=build /go/src/github.com/Interstellarss/faas-share/bin/kubeshare-config-client /usr/bin/kubeshare-config-client

CMD ["kubeshare-config-client", "-alsologtostderr", "-server-ip=kubeshare-device-manager.kube-system.svc:9797", "-v=4"]
