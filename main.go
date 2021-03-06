package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultUpdateURL = "https://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip"
	defaultHashURL   = "https://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip.md5"
)

var (
	host           = flag.String("host", "localhost", "server IP address or hostname")
	port           = flag.Uint("port", 8080, "server port")
	updateInterval = flag.Duration("update-interval", 4*time.Hour, "how often database updates are run")
	updateURL      = flag.String("update-url", defaultUpdateURL, "URL for database updates")
	hashURL        = flag.String("hash-url", "", "URL for checking database hash")
	initURL        = flag.String("init-url", "", "URL for the initial database load")

	dbMux sync.RWMutex
	db    *GeoDB
)

func set(newdb *GeoDB) {
	dbMux.Lock()
	defer dbMux.Unlock()
	db = newdb
}

func get(ip string) (GeoDBEntry, bool) {
	dbMux.RLock()
	defer dbMux.RUnlock()
	return db.Get(ip)
}

func decodeHex(src []byte) ([]byte, error) {
	dst := make([]byte, len(src)/2)
	_, err := hex.Decode(dst, src)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

func tryUpdatingOnce(md5sum []byte, updateSource, hashSource *Source) []byte {
	if hashSource != nil {
		log.Println("Downloading database hash from", hashSource)
		start := time.Now()
		b, err := hashSource.Read()
		if err != nil {
			log.Println("Failed to load database hash:", err)
			return md5sum
		}
		log.Println("Downloaded database hash in", time.Since(start))

		sum, err := decodeHex(b)
		if err != nil {
			log.Println("Couldn't decode hash:", err)
			return md5sum
		}

		if bytes.Equal(md5sum, sum) {
			log.Println("Database not updated (MD5 sums match)")
			return md5sum
		}

		log.Println("Database potentially updated")
	}

	log.Println("Downloading database from", updateSource)
	start := time.Now()
	b, err := updateSource.Read()
	if err != nil {
		log.Println("Failed to load database data:", err)
		return md5sum
	}
	log.Println("Downloaded database in", time.Since(start))

	newsum := md5.Sum(b)
	if bytes.Equal(newsum[:], md5sum) {
		log.Println("Database not updated (MD5 sums match)")
		return md5sum
	}

	log.Println("Parsing database")
	start = time.Now()
	newdb, err := NewGeoDB(b)
	if err != nil {
		log.Println("Failed to parse the database:", err)
		return md5sum
	}
	log.Println("Parsing done in", time.Since(start))

	log.Println("Database updated")
	set(newdb)
	return newsum[:]
}

func update(md5sum []byte, updateSource, hashSource *Source) {
	log.Println("Starting database updates")
	md5sum = tryUpdatingOnce(md5sum, updateSource, hashSource)
	for {
		time.Sleep(*updateInterval)
		md5sum = tryUpdatingOnce(md5sum, updateSource, hashSource)
	}
}

func initial(initSource *Source) ([]byte, error) {
	log.Println("Loading initial database from", initSource)
	data, err := initSource.Read()
	if err != nil {
		return nil, err
	}

	log.Println("Parsing initial database")
	start := time.Now()
	db, err := NewGeoDB(data)
	if err != nil {
		return nil, err
	}
	log.Println("Parsing done in", time.Since(start))
	set(db)

	md5sum := md5.Sum(data)
	return md5sum[:], nil
}

func main() {
	flag.Parse()

	if *hashURL == "" && *updateURL == defaultUpdateURL {
		*hashURL = defaultHashURL
	}
	if *initURL == "" {
		*initURL = *updateURL
	}

	var updateSource, initSource, hashSource *Source
	updateSource, err := NewSource(*updateURL)
	if err != nil {
		log.Fatal(err)
	}
	initSource, err = NewSource(*initURL)
	if err != nil {
		log.Fatal(err)
	}
	if *hashURL != "" {
		hashSource, err = NewSource(*hashURL)
		if err != nil {
			log.Fatal(err)
		}
	}

	md5sum, err := initial(initSource)
	if err != nil {
		log.Fatal(err)
	}
	go update(md5sum[:], updateSource, hashSource)

	http.HandleFunc("/ip/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[4:]
		result, ok := get(path)
		if !ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("{\"message\": \"Not an IPv4/IPv6 address\"}"))
			return
		}
		b, err := json.Marshal(result)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(b)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
		} else {
			http.NotFound(w, r)
		}
	})

	hostPort := net.JoinHostPort(*host, strconv.FormatUint(uint64(*port), 10))
	server := &http.Server{
		Addr:         hostPort,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Println("Serving on", hostPort)
	log.Fatal(server.ListenAndServe())
}
