# build with leeway build :server-docker
FROM alpine:latest

COPY plugins--all/bin/* /app/plugins/
ENV PATH=$PATH:/app/plugins

COPY server/werft /app/werft
RUN chmod +x /app/werft
ENTRYPOINT [ "/app/werft" ]
