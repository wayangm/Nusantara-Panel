package sites

import "testing"

func TestIsValidDomain(t *testing.T) {
	cases := []struct {
		domain string
		ok     bool
	}{
		{domain: "example.com", ok: true},
		{domain: "sub.example.com", ok: true},
		{domain: "-bad.example.com", ok: false},
		{domain: "bad..example.com", ok: false},
		{domain: "localhost", ok: false},
	}
	for _, tc := range cases {
		if got := isValidDomain(tc.domain); got != tc.ok {
			t.Fatalf("domain %q expected %t got %t", tc.domain, tc.ok, got)
		}
	}
}

func TestIsValidRootPath(t *testing.T) {
	cases := []struct {
		root string
		ok   bool
	}{
		{root: "/var/www/site", ok: true},
		{root: "/", ok: false},
		{root: "relative/path", ok: false},
	}
	for _, tc := range cases {
		if got := isValidRootPath(tc.root); got != tc.ok {
			t.Fatalf("root %q expected %t got %t", tc.root, tc.ok, got)
		}
	}
}

