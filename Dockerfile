# Build
FROM golang:1.16-buster AS build
WORKDIR /go/src/github.com/xos/geoip
COPY . .

# Must build without cgo because libc is unavailable in runtime image
ENV GO111MODULE=on CGO_ENABLED=0
RUN make

# Run
FROM scratch
EXPOSE 8080

COPY --from=build /go/bin/geoip /opt/geoip/
COPY html /opt/geoip/html

WORKDIR /opt/geoip
ENTRYPOINT ["/opt/geoip/geoip"]
