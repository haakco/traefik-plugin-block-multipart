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
