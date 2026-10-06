# syntax=docker/dockerfile:1.7
FROM golang:1.26.9-alpine3.23 AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
COPY --from=build --chown=10001:10001 /out/server /usr/local/bin/server
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/livez || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
