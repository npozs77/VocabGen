FROM gcr.io/distroless/static:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/vocabgen /vocabgen

# Data directory for config.yaml and vocabgen.db — container-friendly mount point.
# Users mount a host directory here: docker run -v ./data:/data ...
VOLUME /data
ENV HOME=/home/nonroot

EXPOSE 8080

ENTRYPOINT ["/vocabgen"]
CMD ["serve", "--port", "8080"]
