package main

import (
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
)

type IPv4 uint32

func ParseIPv4(s string) (IPv4, bool) {
	if !strings.ContainsRune(s, '.') {
		return 0, false
	}

	ip := net.ParseIP(s).To4()
	if ip == nil {
		return 0, false
	}

	out := uint32(0)
	for _, num := range ip {
		out <<= 8
		out |= uint32(num)
	}
	return IPv4(out), true
}

func (ip IPv4) masked(bits uint8) IPv4 {
	mask := (uint32(0xffffffff) >> (32 - bits)) << (32 - bits)
	return IPv4(uint32(ip) & mask)
}

type Networks4 struct {
	addrs []IPv4
	bits  []uint8
	ids   []geoNameId
}

func NewNetworks4() *Networks4 {
	return &Networks4{}
}

func (n *Networks4) Append(id geoNameId, s string) error {
	split := strings.SplitN(s, "/", 2)
	if len(split) != 2 {
		return errors.New("not a CIDR")
	}

	ip, ok := ParseIPv4(split[0])
	if !ok {
		return errors.New("couldn't parse the IPv4 address")
	}

	b, err := strconv.ParseUint(split[1], 10, 8)
	if err != nil || b > 32 {
		return errors.New("couldn't parse the bits")
	}

	bits := uint8(b)
	n.addrs = append(n.addrs, ip.masked(bits))
	n.bits = append(n.bits, bits)
	n.ids = append(n.ids, id)
	return nil
}

func (n *Networks4) Len() int {
	return len(n.addrs)
}

func (n *Networks4) Swap(i, j int) {
	n.addrs[i], n.addrs[j] = n.addrs[j], n.addrs[i]
	n.bits[i], n.bits[j] = n.bits[j], n.bits[i]
	n.ids[i], n.ids[j] = n.ids[j], n.ids[i]
}

func (n *Networks4) Less(i, j int) bool {
	return n.addrs[i] < n.addrs[j]
}

func (n *Networks4) Sort() {
	sort.Sort(n)
}

func (n *Networks4) Get(ip IPv4) (geoNameId, bool) {
	idx := sort.Search(len(n.addrs), func(i int) bool {
		return ip < n.addrs[i]
	})
	if idx == 0 {
		return 0, false
	}

	if idx == -1 {
		idx = len(n.addrs)
	}
	idx--
	if n.addrs[idx] == ip.masked(n.bits[idx]) {
		return n.ids[idx], true
	}
	return 0, false
}
