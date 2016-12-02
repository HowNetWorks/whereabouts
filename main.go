package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
)

var urlString = flag.String("url", "http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip", "URL for loading the database")

func readUrl(s string) ([]byte, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "file":
		return ioutil.ReadFile(u.Path)
	case "http", "https":
		resp, err := http.Get(s)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, err
		}

		return ioutil.ReadAll(resp.Body)
	}

	return nil, fmt.Errorf("unknown scheme %q", u.Scheme)
}

type geoNameId uint32

func ParseGeoNameId(n string) (geoNameId, error) {
	i, err := strconv.ParseUint(n, 10, 32)
	if err != nil {
		return 0, err
	}
	return geoNameId(i), nil
}

type continent struct {
	Code string
	Name string
}

type country struct {
	Code string
	Name string
}

type geoNameEntry struct {
	Continent continent
	Country   country
}

type geoNames map[geoNameId]geoNameEntry

func readGeoNames(r io.Reader) (geoNames, error) {
	g := geoNames{}
	c := csv.NewReader(r)
	for i := 0; ; i++ {
		record, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 6 {
			return nil, errors.New("unexpected line")
		}
		if i == 0 {
			continue
		}
		id, err := ParseGeoNameId(record[0])
		if err != nil {
			return nil, err
		}
		g[id] = geoNameEntry{
			Continent: continent{
				Code: record[2],
				Name: record[3],
			},
			Country: country{
				Code: record[4],
				Name: record[5],
			},
		}
	}
	return g, nil
}

func (g geoNames) Get(id geoNameId) (geoNameEntry, bool) {
	e, ok := g[id]
	return e, ok
}

func less(left, right net.IP) bool {
	leftLen := len(left)
	rightLen := len(right)
	if leftLen != rightLen {
		return leftLen < rightLen
	}
	for idx, b := range right {
		if left[idx] != b {
			return left[idx] < b
		}
	}
	return false
}

type network struct {
	net.IPNet
	NameId geoNameId
}

type networks []network

func readNetworks(r io.Reader) (networks, error) {
	n := networks{}

	c := csv.NewReader(r)
	for i := 0; ; i++ {
		record, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 2 {
			return nil, errors.New("unexpected line")
		}
		if i == 0 || record[1] == "" {
			continue
		}

		_, nw, err := net.ParseCIDR(record[0])
		if err != nil {
			return nil, err
		}
		id, err := ParseGeoNameId(record[1])
		if err != nil {
			return nil, err
		}
		n = append(n, network{IPNet: *nw, NameId: id})
	}

	sort.Sort(n)
	return n, nil
}

func (n networks) Len() int {
	return len(n)
}

func (n networks) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n networks) Less(i, j int) bool {
	return less(n[i].IP, n[j].IP)
}

func (n networks) Get(ip net.IP) (geoNameId, bool) {
	idx := sort.Search(len(n), func(i int) bool {
		return less(ip, n[i].IP)
	})
	if idx == 0 {
		return 0, false
	}

	if idx == -1 {
		idx = len(n)
	}
	idx -= 1
	if idx < len(n) && n[idx].IPNet.Contains(ip) {
		return n[idx].NameId, true
	}
	return 0, false
}

type db struct {
	names geoNames
	ipv4  networks
	ipv6  networks
}

func New(b []byte) (*db, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	var names geoNames
	var ipv4 networks
	var ipv6 networks
	for _, f := range z.File {
		name := path.Base(f.Name)

		switch name {
		case "GeoLite2-Country-Blocks-IPv4.csv":
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			ipv4, err = readNetworks(r)
			if err != nil {
				return nil, err
			}
		case "GeoLite2-Country-Blocks-IPv6.csv":
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			ipv6, err = readNetworks(r)
			if err != nil {
				return nil, err
			}
		case "GeoLite2-Country-Locations-en.csv":
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			names, err = readGeoNames(r)
			if err != nil {
				return nil, err
			}
		}
	}

	if names == nil || ipv4 == nil || ipv6 == nil {
		return nil, errors.New("couldn't find all sections")
	}
	return &db{names: names, ipv4: ipv4, ipv6: ipv6}, nil
}

func (d *db) Get(s string) (geoNameEntry, bool) {
	ip := net.ParseIP(s)

	var id geoNameId
	var ok bool
	if ipv4 := ip.To4(); ipv4 != nil {
		id, ok = d.ipv4.Get(ipv4)
	} else if ipv6 := ip.To16(); ipv6 != nil {
		id, ok = d.ipv6.Get(ipv6)
	}

	if !ok {
		return geoNameEntry{}, false
	}
	e, ok := d.names.Get(id)
	return e, ok
}

func main() {
	flag.Parse()

	b, err := readUrl(*urlString)
	if err != nil {
		log.Fatal(err)
	}

	d, err := New(b)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api/ip-to-cc/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[14:]
		result, ok := d.Get(path)
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
	http.ListenAndServe(":8080", nil)
}
