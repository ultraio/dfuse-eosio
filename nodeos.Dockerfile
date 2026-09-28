ARG DFUSE_IMAGE=""
ARG DEB_PKG=""

FROM ${DFUSE_IMAGE}
ARG DEB_PKG
ADD ${DEB_PKG} /tmp/
# fail the build unless nodeos is really installed (the `|| apt-get install -f` fallback alone can exit 0 without it)
RUN apt-get update && (dpkg -i /tmp/${DEB_PKG} || apt-get install -f -y) && nodeos --version
RUN rm -f /tmp/${DEB_PKG} && rm -rf /var/cache/apt/*
