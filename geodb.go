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

type geoNameId uint32

func ParseGeoNameId(n string) (geoNameId, error) {
	i, err := strconv.ParseUint(n, 10, 32)
	if err != nil {
		return 0, err
	}
	return geoNameId(i), nil
}

type continent struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

type country struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

type GeoDBEntry struct {
	Continent continent `json:"continent"`
	Country   country   `json:"country"`
}

type geoNames map[geoNameId]GeoDBEntry

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
		g[id] = GeoDBEntry{
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

type GeoDB struct {
	names geoNames
	ipv4  networks
	ipv6  networks
}

func withEachFile(z *zip.Reader, fn func(string, io.Reader) error) error {
	for _, file := range z.File {
		err := func(f *zip.File) error {
			r, err := f.Open()
			if err != nil {
				return err
			}
			defer r.Close()

			return fn(f.Name, r)
		}(file)

		if err != nil {
			return err
		}
	}
	return nil
}

func NewGeoDB(b []byte) (*GeoDB, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	var ipv4 networks
	var ipv6 networks
	var names geoNames
	withEachFile(z, func(filename string, reader io.Reader) (err error) {
		switch path.Base(filename) {
		case "GeoLite2-Country-Blocks-IPv4.csv":
			ipv4, err = readNetworks(reader)
		case "GeoLite2-Country-Blocks-IPv6.csv":
			ipv6, err = readNetworks(reader)
		case "GeoLite2-Country-Locations-en.csv":
			names, err = readGeoNames(reader)
		}
		return
	})

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
	}

	if !ok {
		return GeoDBEntry{}, false
	}
	e, ok := d.names.Get(id)
	return e, ok
}
