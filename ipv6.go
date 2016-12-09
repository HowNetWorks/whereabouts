package main

import (
	"errors"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
)

type IPv6 struct {
	hi uint64
	lo uint64
}

func ParseIPv6(s string) (IPv6, bool) {
	var out IPv6
	if !strings.ContainsRune(s, ':') {
		return out, false
	}

	ip := net.ParseIP(s).To16()
	if ip == nil {
		return out, false
	}

	for idx, num := range ip {
		if idx < 8 {
			out.hi = (out.hi << 8) | uint64(num)
		} else {
			out.lo = (out.lo << 8) | uint64(num)
		}
	}
	return out, true
}

func (ip IPv6) less(other IPv6) bool {
	if ip.hi != other.hi {
		return ip.hi < other.hi
	}
	return ip.lo < other.lo
}

func (ip IPv6) masked(bits uint8) IPv6 {
	hi := uint8(64)
	lo := uint8(0)
	if bits < 64 {
		hi = bits
	} else {
		lo = bits - 64
	}

	var out IPv6
	out.hi = ip.hi & ((math.MaxUint64 >> (64 - hi)) << (64 - hi))
	out.lo = ip.lo & ((math.MaxUint64 >> (64 - lo)) << (64 - lo))
	return out
}

type Networks6 struct {
	addrs []IPv6
	bits  []uint8
	ids   []geoNameId
}

func NewNetworks6() *Networks6 {
	return &Networks6{}
}

func (n *Networks6) Append(id geoNameId, s string) error {
	split := strings.SplitN(s, "/", 2)
	if len(split) != 2 {
		return errors.New("not a CIDR")
	}

	ip, ok := ParseIPv6(split[0])
	if !ok {
		return errors.New("couldn't parse the IPv6 address")
	}

	b, err := strconv.ParseUint(split[1], 10, 8)
	if err != nil || b > 128 {
		return errors.New("couldn't parse the bits")
	}

	bits := uint8(b)
	n.addrs = append(n.addrs, ip.masked(bits))
	n.bits = append(n.bits, bits)
	n.ids = append(n.ids, id)
	return nil
}

func (n *Networks6) Len() int {
	return len(n.addrs)
}

func (n *Networks6) Swap(i, j int) {
	n.addrs[i], n.addrs[j] = n.addrs[j], n.addrs[i]
	n.bits[i], n.bits[j] = n.bits[j], n.bits[i]
	n.ids[i], n.ids[j] = n.ids[j], n.ids[i]
}

func (n *Networks6) Less(i, j int) bool {
	return n.addrs[i].less(n.addrs[j])
}

func (n *Networks6) Sort() {
	sort.Sort(n)
}

func (n *Networks6) Get(ip IPv6) (geoNameId, bool) {
	idx := sort.Search(len(n.addrs), func(i int) bool {
		return ip.less(n.addrs[i])
	})
	if idx == 0 {
		return 0, false
	}

	if idx == -1 {
		idx = len(n.addrs)
	}
	idx -= 1
	if n.addrs[idx] == ip.masked(n.bits[idx]) {
		return n.ids[idx], true
	}
	return 0, false
}
