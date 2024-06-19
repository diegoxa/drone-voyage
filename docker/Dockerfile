FROM alpine:3.20 as alpine
RUN apk add -U --no-cache ca-certificates openssh
RUN mkdir ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

FROM alpine:3.20
ENV GODEBUG netdns=go

COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine /root/.ssh/known_hosts /root/.ssh/known_hosts

ADD release/linux/amd64/plugin /bin/
ENTRYPOINT ["/bin/plugin"]
