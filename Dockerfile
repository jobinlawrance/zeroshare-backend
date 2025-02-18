# Dockerfile
FROM debian:bullseye-slim

WORKDIR /app

COPY main .
CMD ["./main"]