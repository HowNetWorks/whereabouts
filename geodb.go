package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"path"
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

func (g geoNames) Get(id geoNameId) (GeoDBEntry, bool) {
	e, ok := g[id]
	return e, ok
}

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

func readNetworks4(z *zip.File) (*Networks4, error) {
	n := NewNetworks4()

	err := withEachRecord(z, func(record []string) error {
		if record[1] == "" {
			return nil
		}
		id, err := ParseGeoNameId(record[1])
		if err != nil {
			return err
		}
		return n.Append(id, record[0])
	})
	if err != nil {
		return nil, err
	}

	n.Sort()
	return n, nil
}

func readNetworks6(z *zip.File) (*Networks6, error) {
	n := NewNetworks6()

	err := withEachRecord(z, func(record []string) error {
		if record[1] == "" {
			return nil
		}
		id, err := ParseGeoNameId(record[1])
		if err != nil {
			return err
		}
		return n.Append(id, record[0])
	})
	if err != nil {
		return nil, err
	}

	n.Sort()
	return n, nil
}

type GeoDB struct {
	names geoNames
	ipv4  *Networks4
	ipv6  *Networks6
}

func NewGeoDB(b []byte) (*GeoDB, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	var ipv4 *Networks4
	var ipv6 *Networks6
	var names geoNames
	for _, file := range z.File {
		switch path.Base(file.Name) {
		case "GeoLite2-City-Blocks-IPv4.csv":
			ipv4, err = readNetworks4(file)
		case "GeoLite2-City-Blocks-IPv6.csv":
			ipv6, err = readNetworks6(file)
		case "GeoLite2-City-Locations-en.csv":
			names, err = readGeoNames(file)
		}
		if err != nil {
			return nil, err
		}
	}

	if ipv4 == nil || ipv6 == nil || names == nil {
		return nil, errors.New("couldn't find all sections")
	}
	return &GeoDB{names, ipv4, ipv6}, nil
}

func (d *GeoDB) Get(s string) (GeoDBEntry, bool) {
	var id geoNameId
	var ok bool
	if ipv4, ok4 := ParseIPv4(s); ok4 {
		id, ok = d.ipv4.Get(ipv4)
	} else if ipv6, ok6 := ParseIPv6(s); ok6 {
		id, ok = d.ipv6.Get(ipv6)
	} else {
		return GeoDBEntry{}, false
	}

	if ok {
		return d.names.Get(id)
	}
	return GeoDBEntry{}, true
}
