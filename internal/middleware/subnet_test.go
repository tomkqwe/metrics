package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedSubnet(t *testing.T) {
	for _, tc := range []struct {
		name, cidr string
		values     []string
		allowed    bool
	}{
		{"disabled", "", nil, true},
		{"inside IPv4", "192.0.2.0/24", []string{"192.0.2.255"}, true},
		{"inside IPv6", "2001:db8::/64", []string{"2001:db8::1"}, true},
		{"outside", "192.0.2.0/24", []string{"192.0.3.0"}, false},
		{"missing", "0.0.0.0/0", nil, false},
		{"invalid", "0.0.0.0/0", []string{"invalid"}, false},
		{"multiple", "0.0.0.0/0", []string{"192.0.2.1", "192.0.2.2"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var subnet *net.IPNet
			if tc.cidr != "" {
				var err error
				_, subnet, err = net.ParseCIDR(tc.cidr)
				if err != nil {
					t.Fatal(err)
				}
			}
			req := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			for _, value := range tc.values {
				req.Header.Add("X-Real-IP", value)
			}
			rec := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			WithTrustedSubnet(subnet)(next).ServeHTTP(rec, req)
			want := http.StatusForbidden
			if tc.allowed {
				want = http.StatusNoContent
			}
			if rec.Code != want {
				t.Errorf("status = %d, want %d", rec.Code, want)
			}
		})
	}
}
