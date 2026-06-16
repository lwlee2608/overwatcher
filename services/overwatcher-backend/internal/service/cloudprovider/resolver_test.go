package cloudprovider

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Provider{
		"AS45102 Alibaba (US) Technology Co., Ltd.": Alibaba,
		"AS396982 Google LLC":                       GCP,
		"AS16509 Amazon.com, Inc.":                  AWS,
		"AS14618 Amazon Data Services NoVa":         AWS,
		"AS8075 Microsoft Corporation":              Azure,
		"AS37963 Hangzhou Alibaba Advertising":      Alibaba,
		"AS3320 Deutsche Telekom AG":                Unknown,
		"":                                          Unknown,
	}
	for org, want := range cases {
		if got := classify(org); got != want {
			t.Errorf("classify(%q) = %q, want %q", org, got, want)
		}
	}
}

// Non-public IPs must short-circuit to Unknown without scheduling a lookup, so
// these cases stay network-free.
func TestGetNonPublic(t *testing.T) {
	var nilResolver *Resolver
	r := New("")
	cases := []struct {
		name     string
		resolver *Resolver
		ip       string
	}{
		{"nil resolver", nilResolver, "1.2.3.4"},
		{"empty ip", r, ""},
		{"malformed ip", r, "not-an-ip"},
		{"private ip", r, "10.0.0.1"},
		{"loopback ip", r, "127.0.0.1"},
		{"link-local ip", r, "169.254.1.1"},
	}
	for _, c := range cases {
		if got := c.resolver.Get(c.ip); got != Unknown {
			t.Errorf("%s: Get(%q) = %q, want Unknown", c.name, c.ip, got)
		}
	}
}
