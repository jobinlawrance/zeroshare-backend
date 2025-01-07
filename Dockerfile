FROM golang:1.22 AS builder


# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Copy the cert files into the container
COPY ca.crt ca.key /app/

COPY app-binaries /app-binaries

# Build the application binary
RUN CGO_ENABLED=0 go build -o main .

# Use a lightweight image for the final container
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

WORKDIR /app
# Copy the binary from the builder stage
COPY --from=builder /app/main /app/
COPY --from=builder /app/ca.crt /app/ca.key /app/
COPY --from=builder /app-binaries /app-binaries

RUN ls -l /app-binaries

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


# Expose the application port
EXPOSE 3000

# Command to run the application
CMD ["./main"]