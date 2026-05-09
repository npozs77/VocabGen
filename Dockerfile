FROM alpine:3.23

RUN apk add --no-cache curl su-exec \
    && addgroup -S vocabgen && adduser -S vocabgen -G vocabgen

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/vocabgen /vocabgen
COPY scripts/docker-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Data directory for config.yaml and vocabgen.db — container-friendly mount point.
# Users mount a host directory here: docker run -v ./data:/data ...
VOLUME /data
ENV HOME=/home/vocabgen

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/vocabgen", "serve", "--port", "8080"]
