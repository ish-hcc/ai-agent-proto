# Build stage.
#
# The Go version is pinned to the one in go.mod rather than to "latest", so that
# a rebuild months from now produces the binary this was verified with.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Dependencies are copied on their own so that editing source does not re-download
# the module cache on every rebuild.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is off so the binary runs on a base image with no libc, and the build is
# stripped because the symbol table is only useful where the source is too.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ai-agent-proto ./cmd/ai-agent-proto

# Runtime stage.
#
# alpine rather than scratch: the service reaches CB-Tumblebug and the Messages
# API over TLS and needs the CA bundle, and a shell makes the container
# debuggable, which matters more than the few megabytes for a prototype.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# A non-root user owns the archive directory. The service only ever appends to
# it, so nothing needs to write anywhere else in the image.
RUN adduser -D -u 10001 aiapp
WORKDIR /app
RUN mkdir -p /app/data/archive && chown -R aiapp:aiapp /app

COPY --from=build /out/ai-agent-proto /usr/local/bin/ai-agent-proto

USER aiapp
EXPOSE 8090

# No settings file is baked in. Every setting has a default, so the container
# starts and serves the catalog, the matcher, the tools and the archive with
# nothing supplied; the deployment path needs AIAPP_TUMBLEBUG_BASE_URL and says
# so at startup when it is missing.
ENTRYPOINT ["/usr/local/bin/ai-agent-proto"]
