# Dockerfile
FROM alpine:latest
WORKDIR /app
COPY main .
CMD ["./main"]