# Dockerfile
FROM debian:bullseye-slim

# Install CA certificates for TLS verification
RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /app

COPY main .
CMD ["./main"]