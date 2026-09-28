# Zeabur: ZBPACK_DOCKERFILE_PATH=deploy/api.zeabur.Dockerfile
FROM golang:1.26-alpine AS protocol-assets
WORKDIR /src
COPY go.mod ./
COPY internal/protocol/runtime_manifest.go ./internal/protocol/
COPY tools/fetch-runtime/main.go ./tools/fetch-runtime/
RUN --mount=type=cache,target=/runtime-cache go run ./tools/fetch-runtime --output /runtime-cache && mkdir -p /runtimes && cp /runtime-cache/* /runtimes/

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-amd64 ./cmd/agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-arm64 ./cmd/agent

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 xingdu
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /agents/ /opt/xingdu/agents/
COPY --from=protocol-assets /runtimes/ /opt/xingdu/agents/
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
ENV XINGDU_HTTP_ADDR=0.0.0.0:8080
EXPOSE 8080
USER xingdu
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=4 CMD wget -q -O /dev/null http://127.0.0.1:8080/health/ready || exit 1
CMD ["server"]
