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
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/ ./cmd/...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-amd64 ./cmd/agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-arm64 ./cmd/agent

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 xingdu
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /agents/ /opt/xingdu/agents/
COPY --from=protocol-assets /runtimes/ /opt/xingdu/agents/
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
COPY --chmod=0755 deploy/api-entrypoint.sh /usr/local/bin/xingdu-entrypoint
USER xingdu
ENTRYPOINT ["/usr/local/bin/xingdu-entrypoint"]
CMD ["server"]
