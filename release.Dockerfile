# build with leeway build :server-docker
FROM alpine:latest

COPY werft-server werft /app/
RUN chmod +x /app/werft-server
ENTRYPOINT [ "/app/werft-server" ]
