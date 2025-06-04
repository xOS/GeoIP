# GeoIP

[![Build GeoIP And Push](https://github.com/xOS/GeoIP/actions/workflows/Push.yml/badge.svg)](https://github.com/xOS/GeoIP/actions/workflows/Push.yml)
[![GeoIP Release](https://github.com/xOS/GeoIP/actions/workflows/Build.yml/badge.svg)](https://github.com/xOS/GeoIP/actions/workflows/Build.yml)

A simple service for looking up your IP address. This is the code that powers
https://ifconfig.co.

## 编译后运行

### 使用 MaxMind GeoIP2 数据库
```bash
./geoip -l 0.0.0.0:1212 -a data/GeoLite2-ASN.mmdb -i data/GeoIP2-ISP.mmdb -c data/GeoLite2-City.mmdb -f data/GeoLite2-Country.mmdb -n data/GeoIP2-Connection-Type.mmdb -H x-forwarded-for -r -s -p
```

### 使用 IP2Location IP2PROXY-LITE 数据库（完全替代 MaxMind）
```bash
# CSV 格式
./geoip -l 0.0.0.0:1212 -x data/IP2PROXY-LITE-PX12.CSV -H x-forwarded-for -r -s -p

# BIN 格式（推荐，性能更好）
./geoip -l 0.0.0.0:1212 -x data/IP2PROXY-LITE-PX12.BIN -H x-forwarded-for -r -s -p
```

### 同时使用 MaxMind 和 IP2Proxy 数据库
```bash
./geoip -l 0.0.0.0:1212 -a data/GeoLite2-ASN.mmdb -i data/GeoIP2-ISP.mmdb -c data/GeoLite2-City.mmdb -f data/GeoLite2-Country.mmdb -n data/GeoIP2-Connection-Type.mmdb -x data/IP2PROXY-LITE-PX2.CSV -H x-forwarded-for -r -s -p
```

### 自动检测数据库格式
```bash
# 自动检测所有数据库格式（MMDB、BIN、CSV）
./geoip -auto -f data/GeoLite2-Country.mmdb -x data/IP2PROXY-LITE-PX12.BIN -l 0.0.0.0:1212
```


## Usage

Just the business, please:

```
$ curl ifconfig.co
127.0.0.1

$ http ifconfig.co
127.0.0.1

$ wget -qO- ifconfig.co
127.0.0.1

$ fetch -qo- https://ifconfig.co
127.0.0.1

$ bat -print=b ifconfig.co/ip
127.0.0.1
```

Country and city lookup:

```
$ curl ifconfig.co/country
Elbonia

$ curl ifconfig.co/country-iso
EB

$ curl ifconfig.co/city
Bornyasherk

$ curl ifconfig.co/asn
AS59795
```

Proxy detection and security information:

```
$ curl ifconfig.co/proxy
false

$ curl ifconfig.co/proxy_type
VPN

$ curl ifconfig.co/domain
example.com

$ curl ifconfig.co/usage_type
DCH

$ curl ifconfig.co/threat
BOTNET

$ curl ifconfig.co/fraud_score
95

$ curl ifconfig.co/last_seen
30
```

As JSON:

```
$ curl -H 'Accept: application/json' ifconfig.co  # or curl ifconfig.co/json
{
  "ip": "1.0.0.1",
  "ip_decimal": 16777217,
  "country": "United States",
  "country_code": "US",
  "region": "California",
  "city": "Los Angeles",
  "asn": "AS15169",
  "isp": "Example ISP",
  "org": "Google LLC",
  "isp_org": "Google LLC",
  "isp_asn_org": "Google LLC",
  "isp_asn": "AS15169",
  "connection_type": "DCH",
  "is_proxy": true,
  "proxy_type": "PUB",
  "domain": "example.com",
  "usage_type": "DCH",
  "last_seen": "30",
  "fraud_score": "80",
  "hostname": "one.one.one.one",
  "user_agent": "curl/8.7.1"
}
```

Port testing:

```
$ curl ifconfig.co/port/80
{
  "ip": "127.0.0.1",
  "port": 80,
  "reachable": false
}
```

Pass the appropriate flag (usually `-4` and `-6`) to your client to switch
between IPv4 and IPv6 lookup.

## Features

* Easy to remember domain name
* Fast
* Supports IPv6
* Supports HTTPS
* Supports common command-line clients (e.g. `curl`, `httpie`, `ht`, `wget` and `fetch`)
* JSON output
* Complete geolocation data: country, region, city, ASN, ISP information
* **Multi-format database support**: MMDB, BIN, CSV formats
* Support for both MaxMind GeoIP2 and IP2Location databases
* **Auto-detection** of database formats for seamless integration
* IP2Location databases can completely replace MaxMind databases
* Proxy detection using IP2Location IP2PROXY-LITE database
* Support for multiple proxy types: VPN, TOR, PUB (Public Proxy), DCH (Data Center), WEB, SES, RES, CPN, EPN
* Port testing
* All endpoints (except `/port`) can return information about a custom IP address specified via `?ip=` query parameter
* Open source under the [BSD 3-Clause license](https://opensource.org/licenses/BSD-3-Clause)

## Why?

* To scratch an itch
* An excuse to use Go for something
* Faster than ifconfig.me and has IPv6 support

## Building

Compiling requires the [Golang compiler](https://golang.org/) to be installed.
This package can be installed with `go get`:

`go get github.com/mpolden/geoip/...`

For more information on building a Go project, see the [official Go
documentation](https://golang.org/doc/code.html).

## Docker image

A Docker image is available on [Docker
Hub](https://hub.docker.com/r/mpolden/geoip), which can be downloaded with:

`docker pull mpolden/geoip`

## Deploying to Heroku

Click the button below to automatically build and deploy the application to Heroku

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

### Usage

```
$ geoip -h
Usage of geoip:
  -C int
    	Size of response cache. Set to 0 to disable
  -H value
    	Header to trust for remote IP, if present (e.g. X-Real-IP)
  -a string
    	Path to GeoIP ASN database
  -c string
    	Path to GeoIP City database
  -f string
    	Path to GeoIP Country database
  -i string
    	Path to GeoIP ISP database
  -n string
    	Path to GeoIP Connection-Type database
  -x string
    	Path to IP2Proxy database (CSV/BIN)
  -auto
    	Auto-detect database formats
  -l string
    	Listening address (default ":8080")
  -p	Enable port lookup
  -r	Perform reverse hostname lookups
  -t string
    	Path to template directory (default "html")
```
