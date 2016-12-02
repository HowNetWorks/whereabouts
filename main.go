package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	DEFAULT_UPDATE_URL = "https://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip"
	DEFAULT_HASH_URL   = "https://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip.md5"
)

var updateUrl = flag.String("update-url", DEFAULT_UPDATE_URL, "URL for database updates")
var hashUrl = flag.String("hash-url", "", "URL for checking database hash")
var initUrl = flag.String("init-url", "", "URL for the initial database load")
var dbMux sync.RWMutex
var db *GeoDB

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
		log.Println("Loading database hash from", hashSource)

		b, err := hashSource.Read()
		if err != nil {
			log.Println("Failed to load database hash:", err)
			return md5sum
		}

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

	log.Println("Loading database from", updateSource)
	b, err := updateSource.Read()
	if err != nil {
		log.Println("Failed to load database data:", err)
		return md5sum
	}

	newsum := md5.Sum(b)
	if bytes.Equal(newsum[:], md5sum) {
		log.Println("Database not updated (MD5 sums match)")
		return md5sum
	}

	log.Println("Parsing database")
	newdb, err := NewGeoDB(b)
	if err != nil {
		log.Println("Failed to parse the database:", err)
		return md5sum
	}

	log.Println("Database updated")
	set(newdb)
	return newsum[:]
}

func update(md5sum []byte, updateSource, hashSource *Source) {
	md5sum = tryUpdatingOnce(md5sum, updateSource, hashSource)
	for {
		time.Sleep(1 * time.Hour)
		md5sum = tryUpdatingOnce(md5sum, updateSource, hashSource)
	}
}

func main() {
	flag.Parse()

	if *hashUrl == "" && *updateUrl == DEFAULT_UPDATE_URL {
		*hashUrl = DEFAULT_HASH_URL
	}
	if *initUrl == "" {
		*initUrl = DEFAULT_UPDATE_URL
	}

	var updateSource, initSource, hashSource *Source
	updateSource, err := NewSource(*updateUrl)
	if err != nil {
		log.Fatal(err)
	}
	initSource, err = NewSource(*initUrl)
	if err != nil {
		log.Fatal(err)
	}
	if *hashUrl != "" {
		hashSource, err = NewSource(*hashUrl)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Loading initial database from", initSource)
	b, err := initSource.Read()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Parsing initial database")
	db, err := NewGeoDB(b)
	if err != nil {
		log.Fatal(err)
	}
	set(db)

	log.Println("Starting database updates")
	md5sum := md5.Sum(b)
	go update(md5sum[:], updateSource, hashSource)

	http.HandleFunc("/api/ip-to-cc/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[14:]
		result, ok := get(path)
		if !ok {
			http.NotFound(w, r)
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
	log.Println("Serving on port 8080")
	http.ListenAndServe(":8080", nil)
}
