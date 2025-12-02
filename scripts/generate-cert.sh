#!/bin/bash

# Generate self-signed certificate for development/testing using ECDSA
# ECDSA provides better performance and smaller key size than RSA
# WARNING: Do NOT use self-signed certificates in production!

set -e

CERT_DIR="${1:-./certs}"
DOMAIN="${2:-localhost}"
DAYS=365

echo "🔒 Generating self-signed certificate for development..."
echo "   Domain: $DOMAIN"
echo "   Output directory: $CERT_DIR"
echo ""

# Create directory if it doesn't exist
mkdir -p "$CERT_DIR"

# Generate ECDSA private key (using P-256 curve)
openssl ecparam -genkey -name prime256v1 -out "$CERT_DIR/server.key"

# Generate certificate signing request
openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=$DOMAIN"

# Generate self-signed certificate
openssl x509 -req -days $DAYS \
    -in "$CERT_DIR/server.csr" \
    -signkey "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -extfile <(printf "subjectAltName=DNS:$DOMAIN,DNS:*.$DOMAIN")

# Clean up CSR
rm "$CERT_DIR/server.csr"

echo "✅ Certificate generated successfully!"
echo ""
echo "Files created:"
echo "   Certificate: $CERT_DIR/server.crt"
echo "   Private Key: $CERT_DIR/server.key"
echo ""
echo "Usage:"
echo "   ./bin/drip server \\"
echo "     --domain $DOMAIN \\"
echo "     --tls-cert $CERT_DIR/server.crt \\"
echo "     --tls-key $CERT_DIR/server.key"
echo ""
echo "⚠️  WARNING: This is a self-signed certificate for development only!"
echo "   Clients will need to skip certificate verification (insecure)."
echo "   For production, use --auto-tls or get a certificate from Let's Encrypt."
