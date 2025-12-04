FROM alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375 AS certs

FROM scratch

COPY lancache-dns-sync /lancache-dns-sync
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/lancache-dns-sync"]
