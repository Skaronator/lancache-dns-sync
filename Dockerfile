FROM scratch

COPY lancache-dns-sync /lancache-dns-sync

# Set environment variables
ENV ADGUARD_USERNAME="" \
    ADGUARD_PASSWORD="" \
    LANCACHE_SERVER="" \
    ADGUARD_API="" \
    ALL_SERVICES="" \
    SERVICE_NAMES="" \
    SYNC_INTERVAL_MINUTES="1440"

ENTRYPOINT ["/lancache-dns-sync"]
