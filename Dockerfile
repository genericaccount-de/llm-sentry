FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/sentry ./cmd/sentry

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/sentry /usr/local/bin/sentry
COPY config.example.yaml /etc/llm-sentry/config.yaml
EXPOSE 8080
ENTRYPOINT ["sentry"]
CMD ["-config", "/etc/llm-sentry/config.yaml"]
