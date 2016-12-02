FROM golang:1.7-alpine

COPY . /go/src/ip-to-cc
ADD https://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip /go/src/ip-to-cc
RUN adduser -D app && chown -R app:app /go

USER app
WORKDIR /go/src/ip-to-cc
RUN go build

CMD ./ip-to-cc -init-url file:///go/src/ip-to-cc/GeoLite2-Country-CSV.zip
