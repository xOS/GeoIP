# IP2Proxy Integration Guide

This project now supports IP2Location's databases in multiple formats (CSV, BIN, MMDB) as a complete replacement for MaxMind GeoIP2 databases, providing comprehensive geolocation data including country, region, city, ASN, ISP information, and proxy detection.

## Supported IP2Proxy Databases

The application supports IP2Location IP2PROXY-LITE databases in CSV format:

- **PX1**: IP-Country
- **PX2**: IP-ProxyType-Country  
- **PX3**: IP-ProxyType-Country-Region-City
- **PX4**: IP-ProxyType-Country-Region-City-ISP
- **PX5**: IP-ProxyType-Country-Region-City-ISP-Domain
- **PX6**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType
- **PX7**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN
- **PX8**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN-LastSeen
- **PX9**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN-LastSeen-Threat
- **PX10**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN-LastSeen-Threat-Residential
- **PX11**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN-LastSeen-Threat-Residential-Provider
- **PX12**: IP-ProxyType-Country-Region-City-ISP-Domain-UsageType-ASN-LastSeen-Threat-Residential-Provider-FraudScore

## Proxy Types Detected

The application can detect the following proxy types:

- **VPN**: Anonymizing VPN services
- **TOR**: Tor Exit Nodes
- **DCH**: Data Center/Hosting Provider ranges
- **PUB**: Public Proxies (open proxies)
- **WEB**: Web Proxies
- **SES**: Search Engine Robots
- **RES**: Residential Proxies
- **CPN**: Consumer Privacy Networks
- **EPN**: Enterprise Private Networks

## Download IP2Proxy Database

1. Visit [IP2Location IP2Proxy LITE](https://lite.ip2location.com/ip2proxy-lite)
2. Sign up for a free account
3. Download the desired database (e.g., IP2PROXY-LITE-PX2.CSV)
4. Place the CSV file in your data directory

## Supported Database Formats

### 1. CSV Format
- Human-readable text format
- Easy to edit and debug
- Larger file size
- Slower parsing speed

### 2. BIN Format (Recommended)
- Optimized binary format
- Faster lookup speed
- Smaller file size
- Lower memory usage

### 3. MMDB Format (Future)
- MaxMind-compatible format
- Memory-mapped for efficiency
- Industry standard

## Usage Examples

### Basic Proxy Detection
```bash
# CSV format
./geoip -x data/IP2PROXY-LITE-PX2.CSV -l :8080

# BIN format (recommended)
./geoip -x data/IP2PROXY-LITE-PX2.BIN -l :8080

# Auto-detect format
./geoip -auto -x data/IP2PROXY-LITE-PX2.BIN -l :8080

# Check if IP is a proxy
curl http://localhost:8080/proxy?ip=1.2.3.4

# Get proxy type
curl http://localhost:8080/proxy_type?ip=1.2.3.4

# Get full JSON response
curl http://localhost:8080/json?ip=1.2.3.4
```

### Combined with MaxMind GeoIP2
```bash
# Use both MaxMind and IP2Proxy databases
./geoip \
  -a data/GeoLite2-ASN.mmdb \
  -c data/GeoLite2-City.mmdb \
  -f data/GeoLite2-Country.mmdb \
  -x data/IP2PROXY-LITE-PX2.CSV \
  -l :8080
```

## API Endpoints

### New Proxy Detection Endpoints

- `GET /proxy` - Returns "true" or "false" for proxy detection
- `GET /proxy_type` - Returns the proxy type (VPN, TOR, PUB, etc.)

### Field Data Sources

#### Proxy-related Fields (Smart Display)
These fields are displayed **only when they have content**:

- `proxy_type`: Proxy type (VPN, TOR, PUB, etc.)
- `domain`: Associated domain name
- `usage_type`: Usage classification (DCH, ISP, VPN, etc.)
- `last_seen`: Days since last seen
- `threat`: Threat classification
- `provider`: Service provider name
- `fraud_score`: Fraud risk score

**Smart Display Logic**:
- Fields only appear in JSON when they have meaningful content
- Empty fields ("-" or "") are omitted from the response
- Works in both pure IP2Proxy mode and mixed mode
- Partial data is supported (only fields with content are shown)

#### ISP Field Fallback Logic
The `isp` field uses intelligent fallback:
1. **Primary**: Use ISP field from database
2. **Fallback**: If ISP is empty ("-"), use Organization field
3. **Final fallback**: Use ASN Organization field

### JSON Response Format

When using IP2Proxy, the JSON response includes additional fields:

```json
{
  "ip": "1.2.3.4",
  "ip_decimal": 16909060,
  "country": "United States",
  "country_code": "US",
  "is_proxy": true,
  "proxy_type": "VPN",
  "hostname": "example.com",
  "user_agent": "curl/7.68.0"
}
```

## Data Priority

When both MaxMind and IP2Proxy databases are loaded:

1. **All geolocation data**: IP2Proxy takes priority (country, city, region, ASN, ISP)
2. **Proxy detection**: IP2Proxy data is used exclusively
3. **Fallback**: If IP2Proxy data is unavailable, MaxMind data is used

## Complete Replacement Mode

IP2PROXY-LITE databases (especially PX8-PX12) can completely replace MaxMind databases:

- **Country & Region**: Full country and region information
- **City**: Detailed city-level geolocation
- **ASN & ISP**: Autonomous System Number and ISP details
- **Connection Type**: Usage type classification (DCH, ISP, VPN, etc.)
- **Proxy Detection**: Comprehensive proxy type identification
- **Additional Data**: Domain, threat intelligence, fraud scores (higher levels)

## Performance Considerations

- IP2Proxy CSV databases are loaded into memory for fast lookups
- Binary search is used for efficient IP range matching
- Consider the database size when choosing which IP2Proxy level to use
- PX1-PX2 are suitable for most proxy detection needs
- Higher levels (PX3-PX12) provide more detailed information but larger file sizes

## File Format

The IP2Proxy CSV format expected:
```csv
"ip_from","ip_to","proxy_type","country_code","country_name"
"16777216","16777471","PUB","US","United States"
"16777472","16777727","VPN","US","United States"
```

## Limitations

- Currently supports IPv4 only (IPv6 support planned)
- Basic IP2Proxy levels (PX1-PX2) have limited geographic detail
- CSV parsing requires proper quoting and formatting
- Memory usage scales with database size

## Troubleshooting

### Common Issues

1. **CSV parsing errors**: Ensure proper CSV formatting with quoted fields
2. **Memory usage**: Large databases (PX8+) may require significant RAM
3. **Performance**: Consider using binary format for production (future enhancement)

### Debug Mode

Enable verbose logging to troubleshoot database loading:
```bash
./geoip -x data/IP2PROXY-LITE-PX2.CSV -l :8080 -v
```
