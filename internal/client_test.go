package internal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDo_StatusErrorsWrapSentinels(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		sentinel error
		body     string
	}{
		{"401 unauthorized", http.StatusUnauthorized, ErrUnauthorized, `{"errors":{"auth":"failed"}}`},
		{"403 forbidden", http.StatusForbidden, ErrForbidden, `{"errors":{"perm":"denied"}}`},
		{"400 bad request", http.StatusBadRequest, ErrBadRequest, `{"errors":{"x":"y"}}`},
		{"500 server error", http.StatusInternalServerError, ErrServerError, `internal error`},
		{"503 server error", http.StatusServiceUnavailable, ErrServerError, `down`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := NewProxmoxClient(srv.URL, "tok@pam!t", "secret", false, "info")
			// Override BaseURL: NewProxmoxClient appends /api2/json which the
			// stub server does not need; reset to the bare URL.
			c.BaseURL = srv.URL

			err := c.Get(context.Background(), "/anything", nil)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.sentinel) {
				t.Errorf("err = %v\nwant errors.Is(err, %v) == true", err, tt.sentinel)
			}
		})
	}
}

func TestClientDo_2xxNoSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"release":"8.0.0"}}`))
	}))
	defer srv.Close()

	c := NewProxmoxClient(srv.URL, "tok@pam!t", "secret", false, "info")
	c.BaseURL = srv.URL

	var resp struct {
		Data Version `json:"data"`
	}
	if err := c.Get(context.Background(), "/version", &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.Release != "8.0.0" {
		t.Errorf("Release = %q, want 8.0.0", resp.Data.Release)
	}
}
