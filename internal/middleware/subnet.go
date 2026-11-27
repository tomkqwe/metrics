package middleware

import (
	"net"
	"net/http"
)

// WithTrustedSubnet restricts metric submissions to the configured agent subnet.
func WithTrustedSubnet(subnet *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if subnet == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			values := r.Header.Values("X-Real-IP")
			if len(values) != 1 || !subnet.Contains(net.ParseIP(values[0])) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
