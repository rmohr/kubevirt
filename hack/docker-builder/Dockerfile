FROM fedora@sha256:1b9bfb4e634dc1e5c19d0fa1eb2e5a28a5c2b498e3d3e4ac742bd7f5dae08611

ENV LIBVIRT_VERSION 3.7.0

RUN dnf -y install dnf-plugins-core \
    && dnf -y install libvirt-devel-${LIBVIRT_VERSION} make git mercurial sudo gcc findutils gradle rsync-daemon rsync \
    && dnf -y clean all

RUN dnf -y install dnf-plugins-core \
    && dnf -y copr enable vbatts/bazel \
    && dnf -y install gcc-c++ glibc-devel bazel \
    && dnf -y clean all

ENV GIMME_GO_VERSION=1.9.2

RUN mkdir -p /gimme && curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

ENV GOPATH="/go" GOBIN="/usr/bin"

ADD rsyncd.conf /etc/rsyncd.conf

RUN \
    mkdir -p /go && \
    source /etc/profile.d/gimme.sh && \
    go get github.com/mattn/goveralls && \
    go get -u github.com/Masterminds/glide && \
    go get golang.org/x/tools/cmd/goimports && \
    go get -u mvdan.cc/sh/cmd/shfmt && \
    go get -u github.com/golang/mock/gomock && \
    go get -u github.com/rmohr/mock/mockgen && \
    go get -u github.com/rmohr/go-swagger-utils/swagger-doc && \
    go get -u github.com/onsi/ginkgo/ginkgo && \
    go get -u k8s.io/code-generator/cmd/deepcopy-gen && \
    go get -u k8s.io/code-generator/cmd/defaulter-gen


ADD entrypoint.sh /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]
