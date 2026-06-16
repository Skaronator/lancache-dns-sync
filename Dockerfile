FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS certs

FROM scratch

COPY lancache-dns-sync /lancache-dns-sync
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/lancache-dns-sync"]
