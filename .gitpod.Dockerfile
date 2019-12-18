FROM gitpod/workspace-postgres

USER root
ENV PROTOC_ZIP=protoc-3.7.1-linux-x86_64.zip
RUN curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/$PROTOC_ZIP \
    unzip -o $PROTOC_ZIP -d /usr/local bin/protoc \
    unzip -o $PROTOC_ZIP -d /usr/local 'include/*' \
    rm -f $PROTOC_ZIP
