module github.com/xos/geoip

go 1.23.0

toolchain go1.24.4

require (
	github.com/ip2location/ip2location-go/v9 v9.7.1
	github.com/ip2location/ip2proxy-go/v4 v4.2.0
	github.com/ipipdotnet/ipdb-go v1.3.3
	github.com/oschwald/maxminddb-golang v1.9.0
	github.com/tagphi/czdb-search-golang v1.0.4
	github.com/yinheli/qqwry v0.0.0-20160229183603-f50680010f4a
)

replace github.com/cz88/czdb-search-golang => github.com/tagphi/czdb-search-golang v1.0.0

require (
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yinheli/mahonia v0.0.0-20131226213531-0eef680515cc // indirect
	golang.org/x/sys v0.0.0-20220502124256-b6088ccd6cba // indirect
	lukechampine.com/uint128 v1.2.0 // indirect
)
