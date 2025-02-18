#!/bin/sh
set -e

apt-get update && apt-get install -y wget tar iptables
NEBULA_ARCH=$(dpkg --print-architecture)
echo "Using architecture: ${NEBULA_ARCH}"

if [ ! -f nebula-linux-${NEBULA_ARCH}.tar.gz ]; then
    wget https://github.com/slackhq/nebula/releases/download/v1.9.5/nebula-linux-${NEBULA_ARCH}.tar.gz
fi

if [ ! -f nebula ]; then
    tar -xvf nebula-linux-${NEBULA_ARCH}.tar.gz
fi

cp nebula-cert /shared-bin/
chmod +x /shared-bin/nebula-cert

if [ ! -f /certs/ca.crt ] || [ ! -f /certs/ca.key ]; then
    ./nebula-cert ca -name 'ZeroShare, Inc' -out-crt /certs/ca.crt -out-key /certs/ca.key
    ./nebula-cert sign -name 'lighthouse' -ip '192.168.100.1/24' -groups 'lighthouse' -out-crt /certs/lighthouse.crt -out-key /certs/lighthouse.key -ca-crt /certs/ca.crt -ca-key /certs/ca.key
fi

exec ./nebula -config /config/config.yml