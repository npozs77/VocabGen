FROM alpine:3.23

RUN apk add --no-cache curl \
    && addgroup -S vocabgen && adduser -S vocabgen -G vocabgen

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/vocabgen /vocabgen

# Data directory for config.yaml and vocabgen.db — container-friendly mount point.
# Users mount a host directory here: docker run -v ./data:/data ...
VOLUME /data
ENV HOME=/home/vocabgen

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

USER vocabgen

ENTRYPOINT ["/vocabgen"]
CMD ["serve", "--port", "8080"]
