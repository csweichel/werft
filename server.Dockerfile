# build with leeway build :server-docker
FROM alpine:latest

COPY integrations-plugins-webhook--app/webhook-plugin /app/plugins/werft-plugin-webhook
ENV PATH=$PATH:/app/plugins

COPY server/werft /app/werft
RUN chmod +x /app/werft
ENTRYPOINT [ "/app/werft" ]
