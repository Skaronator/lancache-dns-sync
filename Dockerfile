FROM alpine:3.24.4@sha256:93b41ef40b1677fce8fcc64ff4af09daddfacf5040afde618bc46c1e2d1cc7d4 AS certs

FROM scratch

COPY lancache-dns-sync /lancache-dns-sync
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/lancache-dns-sync"]
