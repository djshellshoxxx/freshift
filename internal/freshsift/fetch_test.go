package freshsift

import (
	"net"
	"testing"
)

func TestRejectsPrivateAndReservedIPs(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "192.168.1.1", "169.254.169.254", "::1", "100.64.0.1", "198.51.100.4"} {
		if safeIP(net.ParseIP(s), false) {
			t.Errorf("allowed blocked IP %s", s)
		}
	}
	if !safeIP(net.ParseIP("1.1.1.1"), false) {
		t.Fatal("blocked public IP")
	}
	if !safeIP(net.ParseIP("127.0.0.1"), true) {
		t.Fatal("allowPrivate didn't apply")
	}
}
func TestRobotsAllowOverrides(t *testing.T) {
	rules := parseRobots("User-agent: *\nDisallow: /private\nAllow: /private/public\n")
	if rules.allowed("/private/secret") {
		t.Fatal("blocked path permitted")
	}
	if !rules.allowed("/private/public/doc") {
		t.Fatal("explicit allowance ignored")
	}
	if !rules.allowed("/other") {
		t.Fatal("allowed page blocked")
	}
}
func TestRobotsWildcardAndEndAnchor(t *testing.T) {
	if !matchRobots("/a/*.pdf$", "/a/b.pdf") || matchRobots("/a/*.pdf$", "/a/b.pdf?x=1") {
		t.Fatal("wildcards or anchors broken")
	}
}
