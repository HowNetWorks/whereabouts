package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net"
	"path"
	"sort"
	"strconv"
)

func withEachRecord(z *zip.File, fn func([]string) error) error {
	r, err := z.Open()
	if err != nil {
		return err
	}
	defer r.Close()

	c := csv.NewReader(r)
	if _, err := c.Read(); err != nil {
		return err
	}

	for i := 0; ; i++ {
		record, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := fn(record); err != nil {
			return err
		}
	}
	return nil
}

type geoNameId uint32

func ParseGeoNameId(n string) (geoNameId, error) {
	i, err := strconv.ParseUint(n, 10, 32)
	if err != nil {
		return 0, err
	}
	return geoNameId(i), nil
}

type Continent struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

func (c Continent) Pointer() *Continent {
	if c.Code == "" && c.Name == "" {
		return nil
	}
	return &c
}

type Country struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

func (c Country) Pointer() *Country {
	if c.Code == "" && c.Name == "" {
		return nil
	}
	return &c
}

type GeoDBEntry struct {
	*Continent `json:"continent,omitempty"`
	*Country   `json:"country,omitempty"`
	City       string `json:"city,omitempty"`
}

type geoNames map[geoNameId]GeoDBEntry

func readGeoNames(z *zip.File) (geoNames, error) {
	g := geoNames{}

	err := withEachRecord(z, func(record []string) error {
		id, err := ParseGeoNameId(record[0])
		if err != nil {
			return err
		}

		continent := Continent{record[2], record[3]}
		country := Country{record[4], record[5]}
		city := record[10]
		g[id] = GeoDBEntry{
			Continent: continent.Pointer(),
			Country:   country.Pointer(),
			City:      city,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g geoNames) Get(id geoNameId) (GeoDBEntry, bool) {
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

func readNetworks(z *zip.File) (networks, error) {
	n := networks{}

	err := withEachRecord(z, func(record []string) error {
		if record[1] == "" {
			return nil
		}

		_, nw, err := net.ParseCIDR(record[0])
		if err != nil {
			return err
		}
		id, err := ParseGeoNameId(record[1])
		if err != nil {
			return err
		}
		n = append(n, network{IPNet: *nw, NameId: id})
		return nil
	})
	if err != nil {
		return nil, err
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

type GeoDB struct {
	names geoNames
	ipv4  networks
	ipv6  networks
}

func NewGeoDB(b []byte) (*GeoDB, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	var ipv4 networks
	var ipv6 networks
	var names geoNames
	for _, file := range z.File {
		switch path.Base(file.Name) {
		case "GeoLite2-City-Blocks-IPv4.csv":
			ipv4, err = readNetworks(file)
		case "GeoLite2-City-Blocks-IPv6.csv":
			ipv6, err = readNetworks(file)
		case "GeoLite2-City-Locations-en.csv":
			names, err = readGeoNames(file)
		}
	}

	if ipv4 == nil || ipv6 == nil || names == nil {
		return nil, errors.New("couldn't find all sections")
	}
	return &GeoDB{names: names, ipv4: ipv4, ipv6: ipv6}, nil
}

func (d *GeoDB) Get(s string) (GeoDBEntry, bool) {
	ip := net.ParseIP(s)

	var id geoNameId
	var ok bool
	if ipv4 := ip.To4(); ipv4 != nil {
		id, ok = d.ipv4.Get(ipv4)
	} else if ipv6 := ip.To16(); ipv6 != nil {
		id, ok = d.ipv6.Get(ipv6)
	} else {
		return GeoDBEntry{}, false
	}

	if ok {
		return d.names.Get(id)
	}
	return GeoDBEntry{}, true
}
