# build with leeway build :server-docker
FROM alpine:latest

COPY plugins--all/bin/* /app/plugins
COPY integrations-plugins-webhook--app/webhook-plugin /app/plugins/werft-plugin-webhook
COPY integrations-plugins-cron--app/cron-plugin /app/plugins/werft-plugin-cron
ENV PATH=$PATH:/app/plugins

COPY server/werft /app/werft
RUN chmod +x /app/werft
ENTRYPOINT [ "/app/werft" ]
