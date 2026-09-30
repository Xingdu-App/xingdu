# One-shot only. Do not deploy as a persistent Zeabur service.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY VERSION ./VERSION
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-X xingdu.app/xingdu/internal/machine.Version=$(cat VERSION)" -o /out/migrate ./cmd/migrate

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 xingdu
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
USER xingdu
CMD ["migrate"]
