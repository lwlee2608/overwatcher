package cloudprovider

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"AS45102 Alibaba (US) Technology Co., Ltd.": "alibaba",
		"AS396982 Google LLC":                       "gcp",
		"AS16509 Amazon.com, Inc.":                  "aws",
		"AS14618 Amazon Data Services NoVa":         "aws",
		"AS8075 Microsoft Corporation":              "azure",
		"AS37963 Hangzhou Alibaba Advertising":      "alibaba",
		"AS3320 Deutsche Telekom AG":                "",
		"":                                          "",
	}
	for org, want := range cases {
		if got := classify(org); got != want {
			t.Errorf("classify(%q) = %q, want %q", org, got, want)
		}
	}
}

func TestGetNilAndEmpty(t *testing.T) {
	var r *Resolver
	if got := r.Get("1.2.3.4"); got != "" {
		t.Errorf("nil resolver Get = %q, want empty", got)
	}
	r = New("")
	if got := r.Get(""); got != "" {
		t.Errorf("empty ip Get = %q, want empty", got)
	}
}
