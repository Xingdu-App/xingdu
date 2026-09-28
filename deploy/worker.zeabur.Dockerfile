# Zeabur: ZBPACK_DOCKERFILE_PATH=deploy/worker.zeabur.Dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-amd64 ./cmd/agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o /agents/xingdu-agent-linux-arm64 ./cmd/agent

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 xingdu
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /agents/ /opt/xingdu/agents/
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
USER xingdu
CMD ["worker"]
