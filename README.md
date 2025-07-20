# GeoIP

[![Build GeoIP And Push](https://github.com/xOS/GeoIP/actions/workflows/Push.yml/badge.svg)](https://github.com/xOS/GeoIP/actions/workflows/Push.yml)
[![GeoIP Release](https://github.com/xOS/GeoIP/actions/workflows/Build.yml/badge.svg)](https://github.com/xOS/GeoIP/actions/workflows/Build.yml)

A simple service for looking up your IP address. This is the code that powers
https://ifconfig.co.

## 🚀 快速开始

**最简单的方式：**

```bash
# 1. 生成配置文件
./geoip -generate-config config.json

# 2. 编辑 config.json 设置你的数据库路径

# 3. 启动服务
./geoip -config config.json
```

**一键启动完整功能：**
```bash
./geoip -config examples/complete-hybrid.json
```

## 编译后运行

### 使用配置文件（推荐）

配置文件方式使复杂的配置变得简单易管理：

```bash
# 生成示例配置文件
./geoip -generate-config my-config.json

# 使用配置文件启动
./geoip -config my-config.json

# 配置文件 + 命令行参数（命令行参数会覆盖配置文件）
./geoip -config my-config.json -l :8080 -C 5000
```

**示例配置文件：**

```json
{
  "listen_address": ":1212",
  "template_dir": "../html",
  "cache_size": 2000,
  "trusted_headers": [
    "X-Real-IP",
    "X-Forwarded-For",
    "CF-Connecting-IP"
  ],
  "enable_reverse_lookup": true,
  "enable_port_lookup": true,
  "enable_profiling": true,
  "databases": {
    "country": "../data/GeoLite2-Country.mmdb",
    "city": "../data/GeoLite2-City.mmdb",
    "asn": "../data/GeoLite2-ASN.mmdb",
    "isp": "../data/GeoIP2-ISP.mmdb",
    "connection_type": "../data/GeoIP2-Connection-Type.mmdb",
    "ip2proxy": "../data/IP2PROXY-LITE-PX12.BIN",
    "qqwry": "../data/qqwry.ipdb"
  },
  "hybrid_mode": true,
  "auto_detect": false
}
```

**路径配置说明：**
- **相对路径**：相对于配置文件所在目录解析（推荐）
- **绝对路径**：直接使用指定的完整路径
- **空路径**：跳过该数据库，不加载

**预设配置示例：**
- `examples/basic-maxmind.json` - 基础 MaxMind 配置
- `examples/hybrid-maxmind-qqwry.json` - MaxMind + qqwry 混合模式
- `examples/complete-hybrid.json` - 完整混合模式（所有数据源）
- `examples/ip2proxy-only.json` - 仅使用 IP2Proxy
- `examples/qqwry-only.json` - 仅使用 qqwry
- `examples/relative-paths.json` - 相对路径示例
- `examples/absolute-paths.json` - 绝对路径示例

详细的路径配置说明请参考：[docs/config-paths.md](docs/config-paths.md)

### 传统命令行方式

如果你偏好命令行参数，以下是各种配置方式：

#### 使用 MaxMind GeoIP2 数据库
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

### 使用 qqwry 数据库（支持 .ipdb 和 .dat 格式）
```bash
# 使用 qqwry.ipdb 数据库（适合中国用户）
./geoip -l 0.0.0.0:1212 -q data/qqwry.ipdb -H x-forwarded-for -r -s -p

# 使用 qqwry.dat 数据库（经典格式，与 .ipdb 功能相同）
./geoip -l 0.0.0.0:1212 -q data/qqwry.dat -H x-forwarded-for -r -s -p
```

### 混合模式：qqwry + MaxMind（推荐）
```bash
# 混合模式：中国大陆IP使用qqwry.ipdb，其他IP使用MaxMind
./geoip -l 0.0.0.0:1212 -hybrid -q data/qqwry.ipdb -c data/GeoLite2-City.mmdb -f data/GeoLite2-Country.mmdb -H x-forwarded-for -r -s -p

# 混合模式：中国大陆IP使用qqwry.dat，其他IP使用MaxMind
./geoip -l 0.0.0.0:1212 -hybrid -q data/qqwry.dat -c data/GeoLite2-City.mmdb -f data/GeoLite2-Country.mmdb -H x-forwarded-for -r -s -p
```

### 完整混合模式：MaxMind + IP2Proxy + qqwry（最强配置）
```bash
# 完整混合模式：集成所有数据源，提供最全面的地理位置和代理检测信息
./geoip -l 0.0.0.0:1212 -hybrid \
  -f data/GeoLite2-Country.mmdb \
  -c data/GeoLite2-City.mmdb \
  -a data/GeoLite2-ASN.mmdb \
  -i data/GeoIP2-ISP.mmdb \
  -n data/GeoIP2-Connection-Type.mmdb \
  -x data/IP2PROXY-LITE-PX12.BIN \
  -q data/qqwry.ipdb \
  -H x-forwarded-for -r -s -p

# 或者使用 qqwry.dat 格式
./geoip -l 0.0.0.0:1212 -hybrid \
  -f data/GeoLite2-Country.mmdb \
  -c data/GeoLite2-City.mmdb \
  -a data/GeoLite2-ASN.mmdb \
  -i data/GeoIP2-ISP.mmdb \
  -n data/GeoIP2-Connection-Type.mmdb \
  -x data/IP2PROXY-LITE-PX12.BIN \
  -q data/qqwry.dat \
  -H x-forwarded-for -r -s -p
```

### 自动检测数据库格式（推荐）
```bash
```bash
# 现在所有参数都支持自动格式检测，可以混合使用不同格式
./geoip -f data/GeoLite2-Country.mmdb -x data/IP2PROXY-LITE-PX12.BIN -l 0.0.0.0:1212

# 自动检测包含qqwry数据库，会自动启用混合模式（支持 .ipdb 和 .dat 格式）
./geoip -auto -f data/GeoLite2-Country.mmdb -c data/GeoLite2-City.mmdb -q data/qqwry.ipdb -l 0.0.0.0:1212

# 使用 qqwry.dat 格式的自动检测
./geoip -auto -f data/GeoLite2-Country.mmdb -c data/GeoLite2-City.mmdb -q data/qqwry.dat -l 0.0.0.0:1212

# IP2Location BIN 可以放在任何参数位置，自动识别
./geoip -f data/IP2LOCATION-LITE-DB1.BIN -c data/GeoLite2-City.mmdb -l 0.0.0.0:1212

# 纯 IP2Location/IP2Proxy 使用
./geoip -x data/IP2PROXY-LITE-PX12.BIN -l 0.0.0.0:1212

# 传统 -auto 参数仍然支持
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
* **Configuration file support**: JSON-based configuration with auto-generation
* **Flexible deployment**: Use config files or command-line arguments (or both)
* Supports IPv6
* Supports HTTPS
* Supports common command-line clients (e.g. `curl`, `httpie`, `ht`, `wget` and `fetch`)
* JSON output
* Complete geolocation data: country, region, city, ASN, ISP information
* **Multi-format database support**: MMDB, BIN, CSV, IPDB, DAT formats
* **Universal auto-detection**: All parameters support automatic format detection
* **Smart fallback**: IP2Location data takes priority, falls back to MaxMind when needed
* **Mixed database support**: Use MaxMind, IP2Location, and qqwry databases together seamlessly
* **QQWry database support**: Support for both .ipdb and .dat formats with intelligent hybrid mode
* **Parameter flexibility**: IP2Location and qqwry databases can be used in any parameter position
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
  -config string
    	Path to configuration file (JSON format)
  -f string
    	Path to GeoIP Country database
  -generate-config string
    	Generate example configuration file and exit
  -i string
    	Path to GeoIP ISP database
  -n string
    	Path to GeoIP Connection-Type database
  -x string
    	Path to IP2Proxy database (CSV/BIN)
  -q string
    	Path to qqwry database (.ipdb or .dat format)
  -hybrid
    	Enable hybrid mode (use qqwry database for China mainland, MaxMind for others)
  -auto
    	Auto-detect database formats
  -l string
    	Listening address (default ":8080")
  -p	Enable port lookup
  -r	Perform reverse hostname lookups
  -t string
    	Path to template directory (default "html")
```
