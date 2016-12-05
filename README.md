# ip-to-cc

A HTTP service for mapping IPv4 and IPv6 addresses to countries and continents.
Written in Go (version 1.7.4).

Uses MaxMind's [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/)
licensed GeoLite2 Country database (available from https://www.maxmind.com/) by
default. However the service can be configured to use any other similarly
formatted database.

The service, once launched, polls the database URL periodically and downloads
updated versions automatically. The service can also be seeded with an initial
database, e.g. for speeding up service startup from a Docker image. See the
included [Dockerfile](./Dockerfile) for an example.

## Command-line Options

 * `port` The port which the HTTP server should listen to. Default: `8080`.
 * `update-interval` How often database updates should be checked from `hash-url`/`update-url`. Uses Go's [Duration format](https://golang.org/pkg/time/#ParseDuration). Default: 4 hours.
 * `update-url` A URL for updating the database. Default: MaxMind's [GeoLite2 Country](https://dev.maxmind.com/geoip/geoip2/geolite2/) database.
 * `hash-url` A URL pointing to a file containing a MD5 sum of the data in `update-url`. Useful for checking whether the database has updated without actually downloading the whole database. Default: Off by default, except when `update-url` points to the its default value.
 * `init-url` A URL for the initial database load. Can be used to seed the service by e.g. baking in a snapshot of the database into the service's a Docker image. Default: The initial load will be performed from `update-url`.

All URL options allow `http`, `https` and `file` URLs.

## API

The service supports requests to `/api/ip-to-cc/IP_ADDRESS` where `IP_ADDRESS`
can be a IPv4 or IPv6 address.

Let's assume the service is running on localhost port 8080 and has done the
initial database load. To query Google's DNS service addresses run the following:

```
$ curl http://locahost:8080/api/ip-to-cc/8.8.8.8
{"Continent":{"Code":"NA","Name":"North America"},"Country":{"Code":"US","Name":"United States"}}
$ curl http://hownetworks.io/api/ip-to-cc/2001:4860:4860::8888
{"Continent":{"Code":"NA","Name":"North America"},"Country":{"Code":"US","Name":"United States"}}
```

If the query if for an address that can't be mapped or isn't a valid IPv4/6
address the service returns status code 404:

```
$ curl http://hownetworks.io/api/ip-to-cc/192.0.2.0
404 page not found
$ curl http://hownetworks.io/api/ip-to-cc/not.an.ip.address
404 page not found
```

GET requests to the root path `/` return the status code 200, but only after the
initial database load has been done. This can be used for service readiness
checks.
