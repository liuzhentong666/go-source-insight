FROM golang:1.25.5-alpine3.22 AS builder
WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal

FROM builder AS api-builder
COPY api ./api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api-server ./api

FROM builder AS cli-builder
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/go-ai-insight ./cmd

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates && addgroup -S -g 10001 insight && adduser -S -D -u 10001 -G insight insight
USER insight

FROM runtime AS api
COPY --from=api-builder /out/api-server /usr/local/bin/api-server
WORKDIR /app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=12 CMD wget -q -O /dev/null http://127.0.0.1:8080/ready || exit 1
ENTRYPOINT ["api-server"]

# The last stage remains the default target and preserves CLI behavior.
FROM runtime AS cli
COPY --from=cli-builder /out/go-ai-insight /usr/local/bin/go-ai-insight
WORKDIR /workspace
ENTRYPOINT ["go-ai-insight"]
CMD ["help"]
