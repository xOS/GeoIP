# Troubleshooting Guide

## Common Issues and Solutions

### 1. BIN File Format Errors

#### Error: "invalid ip_from: strconv.ParseUint: parsing ... invalid syntax"

**Cause**: The application is trying to parse a BIN file as CSV format.

**Solution**: 
- Ensure you're using the latest version with auto-detection
- Use the `-auto` flag for automatic format detection
- Verify the file extension is correct (.bin, .csv, .mmdb)

```bash
# Correct usage for BIN files
./geoip -x data/IP2PROXY-LITE-PX12.BIN -l :8080

# Or use auto-detection
./geoip -auto -x data/IP2PROXY-LITE-PX12.BIN -l :8080
```

#### Error: "failed to open IP2Proxy BIN file: EOF"

**Cause**: The BIN file is empty or corrupted.

**Solution**:
- Download a fresh copy of the database file
- Verify file integrity
- Check file permissions

### 2. Empty Data Results (All Fields Show "-")

#### Error: JSON response shows all fields as "-" or empty

**Example problematic output:**
```json
{
  "ip": "23.80.81.125",
  "country": "-",
  "city": "-",
  "is_proxy": false,
  "proxy_type": "-"
}
```

**Causes and Solutions:**

1. **IP not in database coverage**
   ```bash
   # Use diagnostic tool to check
   ./geoip-debug data/your-database.bin 23.80.81.125

   # Test with known IPs
   curl "http://localhost:8080/json?ip=8.8.8.8"
   ```

2. **Database file is incomplete or corrupted**
   ```bash
   # Check file size
   ls -la data/your-database.bin

   # Re-download the database
   # Verify file integrity
   ```

3. **Wrong database type for your needs**
   - IP2Proxy databases focus on proxy detection
   - IP2Location databases focus on geolocation
   - Choose the right database for your use case

4. **Database version is outdated**
   - Download the latest version
   - Some IP ranges may not be covered in older versions

#### Quick diagnosis:
```bash
# Test multiple IPs to see if database works at all
curl "http://localhost:8080/json?ip=8.8.8.8"    # Google DNS
curl "http://localhost:8080/json?ip=1.1.1.1"    # Cloudflare DNS
curl "http://localhost:8080/json?ip=208.67.222.222"  # OpenDNS
```

### 3. Database Format Detection

#### How to verify format detection:

```bash
# Check what format is detected
./geoip -x your-database-file.bin -l :8080

# Look for log messages indicating the detected format
```

#### Supported file patterns:

| Pattern | Detected As |
|---------|-------------|
| `*.mmdb` | MaxMind MMDB |
| `*ip2proxy*.bin` | IP2Proxy BIN |
| `*proxy*.bin` | IP2Proxy BIN |
| `*.bin` (other) | IP2Location BIN |
| `*ip2proxy*.csv` | IP2Proxy CSV |
| `*proxy*.csv` | IP2Proxy CSV |

### 3. Performance Issues

#### Large CSV files loading slowly:

**Solution**: Use BIN format instead
```bash
# Convert from CSV to BIN (if available from IP2Location)
# Or download BIN format directly
./geoip -x data/IP2PROXY-LITE-PX12.BIN -l :8080
```

#### High memory usage:

**Cause**: CSV files are loaded entirely into memory.

**Solution**:
- Use BIN format for better memory efficiency
- Use lower-level IP2Proxy databases (PX1-PX4 instead of PX8-PX12)

### 4. Database Compatibility

#### Supported IP2Location Products:

✅ **IP2Location DB** (BIN format)
- Country, region, city, ISP, ASN data
- Use with `-f`, `-c`, `-a`, `-i` parameters

✅ **IP2Proxy LITE** (CSV format)
- Proxy detection with geolocation
- Use with `-x` parameter

✅ **IP2Proxy** (BIN format)
- Optimized proxy detection
- Use with `-x` parameter

❌ **Not yet supported**:
- IP2Location MMDB format
- Compressed database files

### 5. Configuration Examples

#### Basic configurations:

```bash
# CSV only (development)
./geoip -x data/IP2PROXY-LITE-PX2.CSV -l :8080

# BIN only (production)
./geoip -x data/IP2PROXY-LITE-PX12.BIN -l :8080

# Mixed MaxMind + IP2Proxy
./geoip -f GeoLite2-Country.mmdb -x IP2PROXY-LITE-PX4.BIN -l :8080

# Auto-detection (recommended)
./geoip -auto -f country.mmdb -x proxy.bin -l :8080
```

#### Advanced configurations:

```bash
# Full MaxMind setup
./geoip \
  -f data/GeoLite2-Country.mmdb \
  -c data/GeoLite2-City.mmdb \
  -a data/GeoLite2-ASN.mmdb \
  -i data/GeoIP2-ISP.mmdb \
  -n data/GeoIP2-Connection-Type.mmdb \
  -l :8080

# IP2Location + IP2Proxy combination
./geoip \
  -f data/IP2LOCATION-LITE-DB1.BIN \
  -x data/IP2PROXY-LITE-PX12.BIN \
  -l :8080
```

### 6. Debugging

#### Use the built-in diagnostic tool:

```bash
# Build the diagnostic tool
go build ./cmd/geoip-debug

# Test your database
./geoip-debug data/IP2PROXY-LITE-PX12.BIN

# Test with specific IP
./geoip-debug data/IP2PROXY-LITE-PX12.BIN 23.80.81.125

# Test format detection
./geoip-debug data/your-database-file.csv
```

#### Manual debugging:

```bash
# Check if file exists and size
ls -la data/your-database-file.bin

# Test with known working IPs
curl "http://localhost:8080/json?ip=8.8.8.8"
curl "http://localhost:8080/json?ip=1.1.1.1"
```

#### Test specific IPs:

```bash
# Test known proxy IPs
curl "http://localhost:8080/json?ip=1.2.3.4"

# Test known clean IPs
curl "http://localhost:8080/json?ip=8.8.8.8"

# Test specific endpoints
curl "http://localhost:8080/proxy?ip=1.2.3.4"
curl "http://localhost:8080/country?ip=1.2.3.4"
```

### 7. File Download and Setup

#### Where to get database files:

1. **IP2Location LITE** (Free):
   - Visit: https://lite.ip2location.com/
   - Download IP2PROXY-LITE-PX1 to PX12
   - Available in CSV and BIN formats

2. **MaxMind GeoLite2** (Free):
   - Visit: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
   - Download GeoLite2-Country, City, ASN
   - MMDB format only

3. **Commercial versions**:
   - IP2Location: https://www.ip2location.com/
   - MaxMind: https://www.maxmind.com/

#### File placement:

```bash
# Recommended directory structure
data/
├── GeoLite2-Country.mmdb
├── GeoLite2-City.mmdb
├── GeoLite2-ASN.mmdb
├── IP2PROXY-LITE-PX12.BIN
└── IP2PROXY-LITE-PX12.CSV
```

### 8. Performance Optimization

#### For production use:

1. **Use BIN format** for best performance
2. **Choose appropriate database level**:
   - PX1-PX2: Basic proxy detection
   - PX4-PX8: Detailed information
   - PX12: Maximum detail (larger file)

3. **Memory considerations**:
   - CSV: ~2-5x file size in memory
   - BIN: ~1-2x file size in memory
   - MMDB: Memory-mapped (most efficient)

#### Monitoring:

```bash
# Check memory usage
ps aux | grep geoip

# Monitor response times
time curl "http://localhost:8080/json?ip=1.2.3.4"

# Load testing
ab -n 1000 -c 10 "http://localhost:8080/json?ip=1.2.3.4"
```

## Getting Help

If you encounter issues not covered here:

1. Check the GitHub issues: https://github.com/xos/geoip/issues
2. Verify your database files are not corrupted
3. Test with the provided sample CSV file first
4. Enable debug logging if available
5. Report bugs with:
   - Database file type and size
   - Command line used
   - Full error message
   - Operating system and Go version
