# Whereabouts [![CircleCI](https://circleci.com/gh/HowNetWorks/whereabouts.svg?style=shield)](https://circleci.com/gh/HowNetWorks/whereabouts)

An HTTP service for mapping IPv4 and IPv6 addresses to countries and continents.
Written in Go (version 1.7.4).

Uses MaxMind's [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/)
licensed GeoLite2 City database (available from https://www.maxmind.com/) by
default. However the service can be configured to use any other similarly
formatted database.

The service, once launched, polls the database URL periodically and downloads
updated versions automatically. The service can also be seeded with an initial
database, e.g. for speeding up service startup from a Docker image.

## Quick Start

Launch Whereabouts as a Docker container:

```sh
$ docker run -ti --rm -p 8080:8080 hownetworks/whereabouts
```

Give the service a moment to download the database. Once that's done you
can start sending queries to localhost port 8080:

```sh
$ curl http://localhost:8080/api/whereabouts/8.8.8.8
```

## Command-line Options

 * `host` The IP address or hostname that the HTTP server should listen to. Default: `localhost`.
 * `port` The port that the HTTP server should listen to. Default: `8080`.
 * `update-interval` How often database updates should be checked from `hash-url`/`update-url`. Uses Go's [Duration format](https://golang.org/pkg/time/#ParseDuration). Default: 4 hours.
 * `update-url` A URL for updating the database. Default: MaxMind's [GeoLite2 City](https://dev.maxmind.com/geoip/geoip2/geolite2/) database.
 * `hash-url` A URL pointing to a file containing an MD5 sum of the data in `update-url`. Useful for checking whether the database has updated without actually downloading the whole database. Default: Off by default, except when `update-url` points to its default value.
 * `init-url` A URL for the initial database load. Can be used to seed the service by e.g. baking in a snapshot of the database into the service's a Docker image. Default: The initial load will be performed from `update-url`.

All URL options allow `http`, `https` and `file` URLs.

## API

The service supports requests to `/api/whereabouts/IP_ADDRESS` where `IP_ADDRESS`
can be an IPv4 or IPv6 address.

Let's assume the service is running on localhost port 8080 and has done the
initial database load. To query Google's DNS service addresses run the following:

```sh
$ curl http://localhost:8080/api/whereabouts/8.8.8.8
{"continent":{"code":"NA","name":"North America"},"country":{"code":"US","name":"United States"},"city":"Mountain View"}
$ curl http://localhost:8080/api/whereabouts/2001:4860:4860::8888
{"continent":{"code":"NA","name":"North America"},"country":{"code":"US","name":"United States"}}
$ curl http://localhost:8080/api/whereabouts/192.0.2.0
{}
```

If the queried IP isn't a valid IPv4/6 address the service returns status code
422 (Unprocessable Entity) with a JSON formatted message object:

```sh
$ curl http://localhost:8080/api/whereabouts/not.an.ip.address
{"message": "Not an IPv4/IPv6 address"}
```

GET requests to the root path `/` return the status code 200, but only after the
initial database load has been done. This can be used for service readiness
checks.
