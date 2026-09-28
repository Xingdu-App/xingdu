FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/ ./cmd/...

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 xingdu
COPY --from=build /out/ /usr/local/bin/
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
USER xingdu
CMD ["server"]
