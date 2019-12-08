# build with leeway build :server-docker
FROM alpine:latest

COPY server/werft /app/werft
RUN chmod +x /app/werft
ENTRYPOINT [ "/app/werft" ]
