FROM gcr.io/distroless/static:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/vocabgen /vocabgen

# Data directory for config.yaml and vocabgen.db
VOLUME /home/nonroot/.vocabgen
ENV HOME=/home/nonroot

EXPOSE 8080

ENTRYPOINT ["/vocabgen"]
CMD ["serve", "--port", "8080"]
