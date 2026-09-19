FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS certs

FROM scratch

COPY lancache-dns-sync /lancache-dns-sync
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/lancache-dns-sync"]
