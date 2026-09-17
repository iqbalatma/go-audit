package audit

import (
	"net"
	"net/http"
	"strings"
)

// RequestInfoFromHTTP extracts RequestInfo from a stdlib *http.Request.
// Reusable from any framework that exposes the underlying *http.Request
// (gin: c.Request, echo: c.Request()).
func RequestInfoFromHTTP(r *http.Request) RequestInfo {
	return RequestInfo{
		IPAddress: clientIP(r),
		Method:    r.Method,
		Endpoint:  r.URL.Path,
		UserAgent: r.UserAgent(),
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware injects RequestInfo into the request context. Works as-is for
// net/http and any net/http-compatible router (chi, gorilla/mux, ...).
//
// gin/echo use their own Context type, not http.Handler, so they can't use
// this directly — call RequestInfoFromHTTP(c.Request) inside your own
// framework middleware instead, then WithRequestInfo the result.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithRequestInfo(r.Context(), RequestInfoFromHTTP(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
