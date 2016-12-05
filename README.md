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
