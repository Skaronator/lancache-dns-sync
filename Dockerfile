FROM alpine:3.22.1 AS certs

FROM scratch

COPY lancache-dns-sync /lancache-dns-sync
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/lancache-dns-sync"]
