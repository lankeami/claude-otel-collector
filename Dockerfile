FROM golang:1.26-alpine AS builder

RUN apk add --no-cache make git sed

# Install ocb
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.115.0 \
    && mv /go/bin/builder /go/bin/ocb

WORKDIR /src
COPY . .

# Generate collector source, patch module name, and build
RUN ocb --config builder-config.yaml --skip-compilation \
    && sed -i 's|module go.opentelemetry.io/collector/cmd/builder|module github.com/jaychinthrajah/claude-otel-collector/cmd/collector|' cmd/collector/go.mod \
    && cd cmd/collector && go build -o /claude-otel-collector .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /claude-otel-collector /claude-otel-collector
COPY config/collector-config.yaml /etc/otel/config.yaml

EXPOSE 4317 4318 13133

ENTRYPOINT ["/claude-otel-collector"]
CMD ["--config", "/etc/otel/config.yaml"]
