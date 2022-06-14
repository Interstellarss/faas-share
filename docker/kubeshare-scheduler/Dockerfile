# call from repo root

FROM ubuntu:18.04 AS build

ENV GOLANG_VERSION 1.18
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/github.com/Interstellarss/faas-share/

COPY . .

RUN apt update && \
    apt-get install -y gnupg1

RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64/3bf863cc.pub
#RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/machine-learning/repos/ubuntu1804/x86_64/7fa2af80.pub

RUN apt update && \
    apt install -y g++ wget make git && \
    wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    make kubeshare-scheduler

FROM alpine:3.9

COPY --from=build /go/src/github.com/Interstellarss/faas-share/bin/kubeshare-scheduler /usr/bin/kubeshare-scheduler

CMD ["kubeshare-scheduler", "-alsologtostderr", "-v=4"]


# docker cp . nifty_maxwell:/go

# export GOLANG_VERSION=1.13.5
# export GOPATH=/go
# export PATH=$GOPATH/bin:/usr/local/go/bin:$PATH
# mkdir -p /go/src/KubeShare
# cd /go/src/KubeShare
