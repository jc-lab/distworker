FROM scratch

ARG DAEMON_NAME="controller"
ARG DIST_FILE=""

RUN mkdir -p /app/
COPY ${DIST_FILE} /app/${DAEMON_NAME}

CMD ["/app/${DAEMON_NAME}"]
