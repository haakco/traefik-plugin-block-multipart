package blockmultipart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type unreadableBody struct{}

func (unreadableBody) Read([]byte) (int, error) { panic("middleware read the body") }
func (unreadableBody) Close() error             { panic("middleware closed the body") }

func TestHeaderOnly(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		for _, values := range [][]string{
			{"multipart/form-data; boundary=x"},
			{"MULTIPART/form-data; boundary=x"},
			{"application/x-multipart-example"},
			{"application/json", "multipart/mixed; boundary=x"},
			nil, {"application/json"}, {"application/x-www-form-urlencoded"},
		} {
			t.Run(method+"/"+http.Header{"Content-Type": values}.Get("Content-Type"), func(t *testing.T) {
				called := false
				handler, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				}), CreateConfig(), "test")
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(method, "http://example.test/", nil)
				req.Header["Content-Type"] = values
				req.Body = unreadableBody{}
				req.ContentLength = 256 << 20
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				blocked := len(values) > 0 && values[0] != "application/json" && values[0] != "application/x-www-form-urlencoded" || len(values) > 1
				want := http.StatusNoContent
				if blocked {
					want = http.StatusForbidden
				}
				if response.Code != want || called == blocked {
					t.Fatalf("status=%d nextCalled=%v; want status=%d nextCalled=%v", response.Code, called, want, !blocked)
				}
			})
		}
	}
}

func TestUploadExceptions(t *testing.T) {
	config := &Config{
		AllowedIPs:     []string{"169.1.17.232", "100.125.0.0/16", "2001:db8::/48"},
		TrustedProxies: []string{"173.245.48.0/20"},
	}
	for _, tc := range []struct {
		name, peer, forwarded string
		allowed               bool
	}{
		{"public IP", "169.1.17.232:1234", "", true},
		{"NetBird range", "100.125.39.60:1234", "", true},
		{"outside range", "100.124.39.60:1234", "", false},
		{"IPv6 range", "[2001:db8::123]:1234", "", true},
		{"spoofed forwarding header", "192.0.2.10:1234", "169.1.17.232", false},
		{"trusted Cloudflare peer", "173.245.48.10:1234", "169.1.17.232", true},
		{"spoofed leftmost hop", "173.245.48.10:1234", "169.1.17.232, 192.0.2.10", false},
		{"trusted chain", "173.245.48.10:1234", "169.1.17.232, 173.245.48.11", true},
		{"invalid forwarded address", "173.245.48.10:1234", "bad-address", false},
		{"missing forwarded address", "173.245.48.10:1234", "", false},
		{"invalid remote address", "bad-address", "169.1.17.232", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}), config, "test")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("PUT", "http://example.test/", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			req.Body = unreadableBody{}
			req.ContentLength = 256 << 20
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			want := http.StatusForbidden
			if tc.allowed {
				want = http.StatusNoContent
			}
			if response.Code != want {
				t.Fatalf("status=%d; want %d", response.Code, want)
			}
		})
	}
}

func TestInvalidRanges(t *testing.T) {
	for _, config := range []*Config{
		{AllowedIPs: []string{""}},
		{AllowedIPs: []string{"192.0.2.1/99"}},
		{TrustedProxies: []string{"not-an-ip"}},
	} {
		if _, err := New(context.Background(), http.NotFoundHandler(), config, "test"); err == nil {
			t.Fatal("invalid range accepted")
		}
	}
}
