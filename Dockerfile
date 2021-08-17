ARG EOSIO_TAG="2.0.9-1.13.0-dm"
ARG DEB_PKG="eosio-2.0.9-1.13.0-dm.deb"

FROM ubuntu:18.04 AS base
ARG EOSIO_TAG
ARG DEB_PKG
RUN apt update && apt-get -y install curl ca-certificates libicu60 libusb-1.0-0 libcurl3-gnutls
RUN mkdir -p /var/cache/apt/archives/
RUN curl -sL -o/var/cache/apt/archives/eosio.deb "https://github-releases.githubusercontent.com/156853671/ef04142a-3123-46a8-8749-6581ae28437d?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAIWNJYAX4CSVEH53A%2F20210817%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20210817T125724Z&X-Amz-Expires=300&X-Amz-Signature=ab25f0a9072c8582f874a6cd19ce8ee81d1e1aa8e7fc8580ad713281867c3a27&X-Amz-SignedHeaders=host&actor_id=57535158&key_id=0&repo_id=156853671&response-content-disposition=attachment%3B%20filename%3Deosio-2.0.9-1.13.0-dm.deb&response-content-type=application%2Foctet-stream"
RUN dpkg -i /var/cache/apt/archives/eosio.deb
RUN rm -rf /var/cache/apt/*

FROM node:12 AS dlauncher
WORKDIR /work
ADD go.mod /work
RUN apt update && apt-get -y install git
RUN cd /work && git clone https://github.com/streamingfast/dlauncher.git dlauncher &&\
	grep -w github.com/streamingfast/dlauncher go.mod | sed 's/.*-\([a-f0-9]*$\)/\1/' |head -n 1 > dlauncher.hash &&\
    cd dlauncher &&\
    git checkout "$(cat ../dlauncher.hash)" &&\
    cd dashboard/client &&\
    yarn install && yarn build

FROM node:12 AS eosq
ADD eosq /work
WORKDIR /work
RUN yarn install && yarn build

FROM golang:1.14 as dfuse
RUN go get -u github.com/GeertJohan/go.rice/rice && export PATH=$PATH:$HOME/bin:/work/go/bin
RUN mkdir -p /work/build
ADD . /work
WORKDIR /work
COPY --from=eosq      /work/ /work/eosq
# The copy needs to be one level higher than work, the dashboard generates expects this file layout
COPY --from=dlauncher /work/dlauncher /dlauncher
RUN cd /dlauncher/dashboard && go generate
RUN cd /work/eosq/app/eosq && go generate
RUN cd /work/dashboard && go generate
RUN cd /work/dgraphql && go generate
RUN go test ./...
RUN go build -v -o /work/build/dfuseeos ./cmd/dfuseeos

FROM base
RUN mkdir -p /app/ && curl -Lo /app/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.2.2/grpc_health_probe-linux-amd64 && chmod +x /app/grpc_health_probe
COPY --from=dfuse /work/build/dfuseeos /app/dfuseeos
COPY --from=dfuse /work/tools/manageos/motd /etc/motd
COPY --from=dfuse /work/tools/manageos/scripts /usr/local/bin/
RUN echo cat /etc/motd >> /root/.bashrc
