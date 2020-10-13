FROM gitpod/workspace-postgres

USER root

ENV PROTOC_ZIP=protoc-3.7.1-linux-x86_64.zip
RUN curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/$PROTOC_ZIP && \
    unzip -o $PROTOC_ZIP -d /usr/local bin/protoc && \
    unzip -o $PROTOC_ZIP -d /usr/local 'include/*' && \
    rm -f $PROTOC_ZIP

RUN curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh
RUN curl -L https://download.docker.com/linux/static/stable/x86_64/docker-19.03.5.tgz | tar xz && \
    mv docker/docker /usr/bin && \
    rm -rf docker

RUN curl -o /usr/bin/k3s -L https://github.com/rancher/k3s/releases/download/v1.0.1/k3s && \
    chmod +x /usr/bin/k3s

RUN curl -L https://github.com/TypeFox/leeway/releases/download/v0.0.8/leeway_0.0.8_Linux_x86_64.tar.gz | tar xz && \
    mv leeway /usr/bin/leeway && \
    rm README.md

RUN go get golang.org/dl/go1.15 && \
    go1.15 download && \
    mv $(which go1.15) $(which go)