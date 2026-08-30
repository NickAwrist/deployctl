package internal

import "testing"

func TestHTTPAddr(t *testing.T) {
	t.Run("uses loopback by default", func(t *testing.T) {
		t.Setenv("DEPLOYCTL_HTTP_ADDR", "")
		if got := HTTPAddr(); got != "127.0.0.1:7123" {
			t.Fatalf("expected loopback address, got %q", got)
		}
	})

	t.Run("uses configured address", func(t *testing.T) {
		t.Setenv("DEPLOYCTL_HTTP_ADDR", "0.0.0.0:9000")
		if got := HTTPAddr(); got != "0.0.0.0:9000" {
			t.Fatalf("expected configured address, got %q", got)
		}
	})
}
