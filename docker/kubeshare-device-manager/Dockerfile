# call from repo root

FROM ubuntu:18.04 AS build

ENV GOLANG_VERSION 1.18
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/github.com/Interstellarss/faas-share

COPY . .

RUN apt update && \
    apt-get install -y gnupg1

RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64/3bf863cc.pub
#RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/machine-learning/repos/ubuntu1804/x86_64/7fa2af80.pub

RUN apt update && \
    apt install -y g++ wget make git && \
    wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    make kubeshare-device-manager

FROM alpine:3.9

COPY --from=build /go/src/github.com/Interstellarss/faas-share/bin/kubeshare-device-manager /usr/bin/kubeshare-device-manager

CMD ["kubeshare-device-manager", "-alsologtostderr", "-v=4"]
