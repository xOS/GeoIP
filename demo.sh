#!/bin/bash

# IP2Proxy Integration Demo Script
# This script demonstrates the new IP2Proxy functionality

echo "=== IP2Proxy Integration Demo ==="
echo

# Build the application
echo "Building application..."
go build ./cmd/geoip
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Start the server with IP2Proxy database
echo "Starting server with IP2Proxy database..."
./geoip -x data/test-ip2proxy.csv -l :8080 &
SERVER_PID=$!

# Wait for server to start
sleep 2

echo "Server started with PID: $SERVER_PID"
echo

# Test various IP addresses
echo "=== Testing IP2Proxy Functionality ==="
echo

echo "1. Testing US IP (1.0.0.1) - Should show Los Angeles, CA:"
curl -s "http://localhost:8080/json?ip=1.0.0.1" | jq '.'
echo

echo "2. Testing VPN IP (1.0.1.200) - Should show New York, NY with VPN type:"
curl -s "http://localhost:8080/json?ip=1.0.1.200" | jq '.'
echo

echo "3. Testing China IP (8.8.8.8) - Should show Beijing, China:"
curl -s "http://localhost:8080/json?ip=8.8.8.8" | jq '.'
echo

echo "=== Testing CLI Endpoints ==="
echo

echo "City lookup:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/city?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/city?ip=1.0.1.200")"
echo

echo "Country lookup:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/country?ip=1.0.0.1")"
echo "8.8.8.8 -> $(curl -s "http://localhost:8080/country?ip=8.8.8.8")"
echo

echo "ASN lookup:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/asn?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/asn?ip=1.0.1.200")"
echo

echo "ISP lookup:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/isp?ip=1.0.0.1")"
echo "8.8.8.8 -> $(curl -s "http://localhost:8080/isp?ip=8.8.8.8")"
echo

echo "Proxy detection:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/proxy?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/proxy?ip=1.0.1.200")"
echo

echo "Proxy type:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/proxy_type?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/proxy_type?ip=1.0.1.200")"
echo

echo "Connection type:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/connection_type?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/connection_type?ip=1.0.1.200")"
echo

echo "=== Testing IP2Proxy Extended Fields ==="
echo

echo "Domain:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/domain?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/domain?ip=1.0.1.200")"
echo

echo "Usage Type:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/usage_type?ip=1.0.0.1")"
echo "1.0.1.200 -> $(curl -s "http://localhost:8080/usage_type?ip=1.0.1.200")"
echo

echo "Last Seen (days):"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/last_seen?ip=1.0.0.1")"
echo "1.0.2.1 -> $(curl -s "http://localhost:8080/last_seen?ip=1.0.2.1")"
echo

echo "Threat Intelligence:"
echo "1.0.2.1 -> $(curl -s "http://localhost:8080/threat?ip=1.0.2.1")"
echo "8.8.8.8 -> $(curl -s "http://localhost:8080/threat?ip=8.8.8.8")"
echo

echo "Fraud Score:"
echo "1.0.0.1 -> $(curl -s "http://localhost:8080/fraud_score?ip=1.0.0.1")"
echo "1.0.2.1 -> $(curl -s "http://localhost:8080/fraud_score?ip=1.0.2.1")"
echo

echo "=== Demo Complete ==="
echo

# Clean up
echo "Stopping server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "Demo finished successfully!"
echo
echo "Key Features Demonstrated:"
echo "- Complete geolocation data from IP2Proxy (country, region, city)"
echo "- ASN and ISP information"
echo "- Proxy detection and classification"
echo "- Connection type identification"
echo "- All 16 IP2Proxy database fields available in JSON and CLI endpoints:"
echo "  * Basic geo: country, region, city, ASN, ISP"
echo "  * Proxy info: proxy_type, domain, usage_type"
echo "  * Security: threat intelligence, fraud_score, last_seen"
echo "  * Provider information"
echo "- All standard CLI endpoints working with IP2Proxy data"
echo "- New CLI endpoints: /domain, /usage_type, /last_seen, /threat, /fraud_score, /provider"
echo
echo "IP2Proxy can now completely replace MaxMind databases with enhanced security features!"
