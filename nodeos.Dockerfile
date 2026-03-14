ARG DFUSE_IMAGE=""
ARG DEB_PKG=""

FROM ${DFUSE_IMAGE}
ARG DEB_PKG
ADD ${DEB_PKG} /tmp/
RUN apt-get update && dpkg -i /tmp/${DEB_PKG} || apt-get install -f -y
RUN rm -f /tmp/${DEB_PKG} && rm -rf /var/cache/apt/*
