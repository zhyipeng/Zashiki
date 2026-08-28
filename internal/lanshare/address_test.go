package lanshare

import (
	"net"
	"strings"
	"testing"
)

func TestPrivateRank(t *testing.T) {
	cases := []struct {
		ip   string
		rank int
	}{
		{"192.168.1.5", 0},
		{"10.0.0.2", 1},
		{"172.16.0.1", 2},
		{"100.64.0.1", 3},
		{"8.8.8.8", 3},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip).To4()
		if ip == nil {
			t.Fatalf("bad test ip %q", c.ip)
		}
		if got := privateRank(ip); got != c.rank {
			t.Errorf("privateRank(%s) = %d, want %d", c.ip, got, c.rank)
		}
	}
}

func TestPrivateRankOrdering(t *testing.T) {
	home := net.ParseIP("192.168.1.5").To4()
	corp := net.ParseIP("10.1.2.3").To4()
	other := net.ParseIP("172.20.1.2").To4()
	public := net.ParseIP("203.0.113.9").To4()
	if !(privateRank(home) < privateRank(corp) && privateRank(corp) < privateRank(other) && privateRank(other) < privateRank(public)) {
		t.Error("private ranks should be strictly ordered 192.168 < 10 < 172.16/12 < other")
	}
}

func TestLanURLsExcludesLoopbackAndCarriesToken(t *testing.T) {
	urls := lanURLs(53100, "tok123")
	if len(urls) == 0 {
		t.Skip("no non-loopback IPv4 interface on this machine")
	}
	for _, u := range urls {
		if !strings.HasPrefix(u, "http://") {
			t.Errorf("url %q should be http", u)
		}
		if !strings.Contains(u, ":53100/tok123/") {
			t.Errorf("url %q missing port and token", u)
		}
		if strings.HasPrefix(u, "http://127.") || strings.HasPrefix(u, "http://::1") {
			t.Errorf("url %q must not be loopback", u)
		}
	}
	// Sorted: every private URL precedes any non-private one.
	firstPublic := -1
	for i, u := range urls {
		hostPort := strings.TrimPrefix(u, "http://")
		host := hostPort[:strings.Index(hostPort, ":")]
		ip := net.ParseIP(host)
		if ip == nil {
			t.Fatalf("unparseable host %q in %q", host, u)
		}
		if privateRank(ip) >= 3 && firstPublic < 0 {
			firstPublic = i
		}
		if firstPublic >= 0 && privateRank(ip) < 3 {
			t.Errorf("private url %q sorted after a public one", u)
		}
	}
}
