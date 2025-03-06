#!/bin/sh
set -e

# Enable debug logging
set -x

echo "Starting Nebula Lighthouse setup..."

# Check if required directories exist
for dir in "/shared-bin" "/certs" "/config"; do
    if [ ! -d "$dir" ]; then
        echo "Error: Required directory $dir does not exist"
        exit 1
    fi
done

echo "Installing required packages..."
apt-get update && apt-get install -y wget tar iptables || {
    echo "Failed to install required packages"
    exit 1
}

NEBULA_ARCH=$(dpkg --print-architecture)
echo "Using architecture: ${NEBULA_ARCH}"

echo "Downloading Nebula binary if needed..."
if [ ! -f nebula-linux-${NEBULA_ARCH}.tar.gz ]; then
    wget https://github.com/slackhq/nebula/releases/download/v1.9.5/nebula-linux-${NEBULA_ARCH}.tar.gz || {
        echo "Failed to download Nebula binary"
        exit 1
    }
fi

echo "Extracting Nebula binary if needed..."
if [ ! -f nebula ]; then
    tar -xvf nebula-linux-${NEBULA_ARCH}.tar.gz || {
        echo "Failed to extract Nebula archive"
        exit 1
    }
fi

echo "Copying nebula-cert to shared bin..."
cp nebula-cert /shared-bin/ || {
    echo "Failed to copy nebula-cert to shared bin"
    exit 1
}
chmod +x /shared-bin/nebula-cert || {
    echo "Failed to make nebula-cert executable"
    exit 1
}

echo "Checking and generating certificates if needed..."
if [ ! -f /certs/ca.crt ] || [ ! -f /certs/ca.key ]; then
    echo "Generating CA certificate..."
    ./nebula-cert ca -name 'ZeroShare, Inc' -out-crt /certs/ca.crt -out-key /certs/ca.key || {
        echo "Failed to generate CA certificate"
        exit 1
    }
    
    echo "Generating lighthouse certificate..."
    ./nebula-cert sign -name 'lighthouse' -ip '192.168.100.1/24' -groups 'lighthouse' \
        -out-crt /certs/lighthouse.crt -out-key /certs/lighthouse.key \
        -ca-crt /certs/ca.crt -ca-key /certs/ca.key || {
        echo "Failed to generate lighthouse certificate"
        exit 1
    }
fi

# Verify config file exists
if [ ! -f /config/config.yml ]; then
    echo "Error: Config file not found at /config/config.yml"
    exit 1
fi

# Verify certificates exist before starting
for cert in "/certs/ca.crt" "/certs/ca.key" "/certs/lighthouse.crt" "/certs/lighthouse.key"; do
    if [ ! -f "$cert" ]; then
        echo "Error: Required certificate $cert not found"
        exit 1
    fi
done

echo "Starting Nebula lighthouse..."
exec ./nebula -config /config/config.yml