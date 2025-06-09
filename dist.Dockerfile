FROM alpine:3.21

ARG DAEMON_NAME="controller"
ARG DIST_FILE=""

RUN mkdir -p /app/
COPY ${DIST_FILE} /app/${DAEMON_NAME}

CMD ["/app/${DAEMON_NAME}"]
