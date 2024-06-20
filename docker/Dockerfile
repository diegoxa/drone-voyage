FROM alpine:3.20 as alpine
RUN apk add -U --no-cache ca-certificates openssh
RUN mkdir ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

FROM golang:1.22-alpine3.20 as build
ENV GO111MODULE=on
ENV CGO_ENABLED=0
WORKDIR /app

COPY . .
RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o release/linux/plugin

FROM alpine:3.20
ENV GODEBUG netdns=go

COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine /root/.ssh/known_hosts /root/.ssh/known_hosts
COPY --from=build /app/release/linux/plugin /bin/

ENTRYPOINT ["/bin/plugin"]
