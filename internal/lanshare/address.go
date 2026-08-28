package lanshare

import (
	"net"
	"sort"
	"strconv"
)

// lanURLs returns candidate share URLs, one per non-loopback IPv4 address,
// private-range addresses first. IPv6 is skipped for MVP: link-local addresses
// need a zone suffix that QR scanners and URL bars handle poorly.
func lanURLs(port int, token string) []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var ips []net.IP
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
			continue
		}
		ips = append(ips, ip)
	}
	sort.SliceStable(ips, func(i, j int) bool {
		return privateRank(ips[i]) < privateRank(ips[j])
	})
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, "http://"+ip.String()+":"+strconv.Itoa(port)+"/"+token+"/")
	}
	return urls
}

// privateRank orders addresses by how likely they are the LAN the receiver is
// on: RFC1918 ranges first (192.168 beats 10. beats 172.16-31, matching home
// router prevalence), then everything else.
func privateRank(ip net.IP) int {
	switch {
	case ip.IsPrivate():
		switch {
		case ip[0] == 192:
			return 0
		case ip[0] == 10:
			return 1
		default:
			return 2
		}
	default:
		return 3
	}
}
