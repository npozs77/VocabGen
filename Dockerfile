# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
ARG VERSION=dev

COPY . .
RUN cp CHANGELOG.md docs/changelog.md
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o /vocabgen ./cmd/vocabgen

# Runtime stage
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /vocabgen /vocabgen

# Data directory for config.yaml and vocabgen.db
VOLUME /home/nonroot/.vocabgen
ENV HOME=/home/nonroot

EXPOSE 8080

ENTRYPOINT ["/vocabgen"]
CMD ["serve", "--port", "8080"]
