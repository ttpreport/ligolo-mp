#!/bin/bash

if [[ "$EUID" -ne 0 ]]; then
    echo "Please run as root"
    exit
fi

if command -v apt-get &> /dev/null; then # Debian-based OS (Debian, Ubuntu, etc)
    echo "Installing dependencies using apt..."
    DEBIAN_FRONTEND=noninteractive apt-get install -yqq \
        gpg curl build-essential git \
        mingw-w64 binutils-mingw-w64 g++-mingw-w64
        INSTALLER=(apt-get install -yqq)
elif command -v yum &> /dev/null; then # Redhat-based OS (Fedora, CentOS, RHEL)
    echo "Installing dependencies using yum..."
    yum -y install gnupg curl gcc gcc-c++ make mingw64-gcc git
        INSTALLER=(yum -y)
elif command -v pacman &>/dev/null; then # Arch-based (Manjaro, Garuda, Blackarch)
        echo "Installing dependencies using pacman..."
        pacman --noconfirm -S mingw-w64-gcc mingw-w64-binutils mingw-w64-headers
    INSTALLER=(pacman --noconfirm -S)
else
    echo "Unsupported OS, exiting"
    exit
fi

# Verify if necessary tools are installed
for cmd in curl awk gpg; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "$cmd could not be found, installing..."
                ${INSTALLER[@]} "$cmd"
    fi
done

ARCH=$(uname -m)
case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="arm";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
esac

cd /root || exit
echo "Running from $(pwd)"

echo "Fetching latest ligolo-mp release..."
ARTIFACTS=$(curl -s "https://api.github.com/repos/ttpreport/ligolo-mp/releases/latest" | awk -F '"' '/browser_download_url/{print $4}')
SERVER_BINARY="ligolo-mp_server_linux_${ARCH}"
CLIENT_BINARY="ligolo-mp_client_linux_${ARCH}"
CHECKSUMS_FILE="ligolo-mp_checksums.txt"


for URL in $ARTIFACTS
do
    if [[ "$URL" == *"$SERVER_BINARY"* ]]; then
        echo "Downloading $URL"
        curl --silent -L "$URL" --output "$(basename "$URL")"
    fi
    if [[ "$URL" == *"$CLIENT_BINARY"* ]]; then
        echo "Downloading $URL"
        curl --silent -L "$URL" --output "$(basename "$URL")"
    fi
    if [[ "$URL" == *"$CHECKSUMS_FILE"* ]]; then
        echo "Downloading $URL"
        curl --silent -L "$URL" --output "$(basename "$URL")"
    fi
done

# Signature verification
echo "Verifying signatures ..."
sha256sum --ignore-missing -c ligolo-mp_checksums.txt || (echo "Signature mismatch! Aborting..." && exit 2)

if test -f "/root/$SERVER_BINARY"; then
    echo "Moving the server executable to /root/ligolo-mp-server..."
    mv "/root/$SERVER_BINARY" /root/ligolo-mp-server

    echo "Setting permissions for the server executable..."
    chmod 755 /root/ligolo-mp-server
else
    echo "$SERVER_BINARY not found! Aborting..." 
    exit 3
fi

if test -f "/root/$CLIENT_BINARY"; then
    echo "Copying the client executable to /usr/local/bin/ligolo-mp-client..."
    mv "/root/$CLIENT_BINARY" /usr/local/bin/ligolo-mp-client

    echo "Setting permissions for the client executable..."
    chmod 755 "/root/$CLIENT_BINARY"

    echo "Creating a symbolic link for client at /usr/local/bin/ligolo-mp..."
    ln -sf /usr/local/bin/ligolo-mp-client /usr/local/bin/ligolo-mp

    echo "Setting permissions for the symbolic link /usr/local/bin/ligolo-mp..."
    chmod 755 /usr/local/bin/ligolo-mp
else
    echo "$CLIENT_BINARY not found! Aborting..." 
    exit 3
fi

echo "Stopping Ligolo-mp service..."
systemctl stop ligolo-mp

echo "Unpacking server files..."
/root/ligolo-mp-server -unpack

echo "Initializing operators..."
echo -n "IP to reach the server [e.g. 10.10.20.25]: "
read SERVER_ADDR
/root/ligolo-mp-server -init-operators -operator-addr $SERVER_ADDR:58008

# systemd
echo "Configuring systemd service ..."
cat > /etc/systemd/system/ligolo-mp.service <<-EOF
[Unit]
Description=Ligolo-mp
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=on-failure
RestartSec=3
User=root
ExecStart=/root/ligolo-mp-server -daemon

[Install]
WantedBy=multi-user.target
EOF

chown root:root /etc/systemd/system/ligolo-mp.service
chmod 600 /etc/systemd/system/ligolo-mp.service

echo "Starting the Ligolo-mp service..."
systemctl start ligolo-mp