package audit

import (
	"net/http"
	"testing"
)

func TestClientIPPrefersForwardedFor(t *testing.T) {
	r := &http.Request{Header: http.Header{"X-Forwarded-For": []string{"1.2.3.4, 5.6.7.8"}}, RemoteAddr: "9.9.9.9:1234"}
	if ip := clientIP(r); ip != "1.2.3.4" {
		t.Fatalf("clientIP = %q, want 1.2.3.4", ip)
	}
}

func TestClientIPFallsBackToRemoteAddr(t *testing.T) {
	r := &http.Request{Header: http.Header{}, RemoteAddr: "9.9.9.9:1234"}
	if ip := clientIP(r); ip != "9.9.9.9" {
		t.Fatalf("clientIP = %q, want 9.9.9.9", ip)
	}
}
