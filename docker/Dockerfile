# syntax=docker/dockerfile:1

# Cross-compiled on the build host's native platform ($BUILDPLATFORM) so
# multi-arch builds don't need QEMU to run the Go toolchain: Go's own
# cross-compilation (CGO disabled) produces the target's binary directly.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine3.23 AS build
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ENV GO111MODULE=on
ENV CGO_ENABLED=0
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -o release/plugin

FROM alpine:3.23 AS alpine
RUN apk add -U --no-cache ca-certificates openssh
RUN mkdir ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

FROM alpine:3.23
ENV GODEBUG netdns=go

COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine /root/.ssh/known_hosts /root/.ssh/known_hosts
COPY --from=build /app/release/plugin /bin/

ENTRYPOINT ["/bin/plugin"]
