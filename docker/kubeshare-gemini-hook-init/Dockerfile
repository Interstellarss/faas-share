# call from repo root

FROM nvidia/cuda:10.0-devel AS build

WORKDIR /tmp/build

RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64/3bf863cc.pub
RUN apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/machine-learning/repos/ubuntu1804/x86_64/7fa2af80.pub

RUN apt update && \
    apt-get install libzmq3-dev && \
    apt install -y git && \
    git clone https://github.com/NTHU-LSALAB/Gemini.git && cd Gemini && \
    make -C src

# FROM busybox:1.31.1-glibc
FROM alpine:3.9

COPY --from=build /tmp/build/Gemini/lib/libgemhook.so.1 /libgemhook.so.1

CMD ["cp", "/libgemhook.so.1", "/faas-share/library/libgemhook.so.1"]
