# Dockerfile
FROM alpine:latest
WORKDIR /app

COPY --from=builder /app-binaries /app-binaries

# Add the correct nebula and nebula-cert binaries based on the platform
ARG TARGETPLATFORM
RUN case "$TARGETPLATFORM" in \
      "linux/amd64") cp /app-binaries/nebula-linux-amd64/nebula /usr/local/bin/nebula && \
                     cp /app-binaries/nebula-linux-amd64/nebula-cert /usr/local/bin/nebula-cert ;; \
      "linux/arm64") cp /app-binaries/nebula-linux-arm64/nebula /usr/local/bin/nebula && \
                     cp /app-binaries/nebula-linux-arm64/nebula-cert /usr/local/bin/nebula-cert ;; \
      *) echo "Unsupported platform: $TARGETPLATFORM" && exit 1 ;; \
    esac && \
    chmod +x /usr/local/bin/nebula /usr/local/bin/nebula-cert

COPY main .
CMD ["./main"]