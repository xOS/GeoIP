FROM golang:alpine AS binarybuilder
RUN apk --no-cache --no-progress add \
    gcc git musl-dev
WORKDIR /geoip
COPY . .
RUN cd cmd/geoip && go build -o app -ldflags="-s -w"

FROM alpine:latest
ENV TZ="Asia/Shanghai"
RUN apk --no-cache --no-progress add \
    ca-certificates \
    tzdata && \
    cp "/usr/share/zoneinfo/$TZ" /etc/localtime && \
    echo "$TZ" >  /etc/timezone
WORKDIR /geoip
COPY ./data ./data
COPY ./html ./html
COPY --from=binarybuilder /geoip/cmd/geoip/app ./app

VOLUME ["/geoip/data"]
EXPOSE 8008 1212
CMD ["/geoip/app -a data/GeoLite2-ASN.mmdb -i data/GeoIP2-ISP.mmdb -c data/GeoLite2-City.mmdb -f data/GeoLite2-Country.mmdb -n data/GeoIP2-Connection-Type.mmdb -H x-forwarded-for -r -s -p"]
