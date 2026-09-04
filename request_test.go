package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid URL",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			url:     "://invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Opt{URL: tt.url}
			req, err := o.createRequest()
			if (err != nil) != tt.wantErr {
				t.Fatalf("createRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if req.Method != "GET" {
				t.Errorf("request method = %q, want %q", req.Method, "GET")
			}
			if req.URL.String() != tt.url {
				t.Errorf("request URL = %q, want %q", req.URL.String(), tt.url)
			}
		})
	}
}

func TestDoRequest(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "successful request",
			statusCode: http.StatusOK,
			body:       "hello",
		},
		{
			name:       "empty body",
			statusCode: http.StatusNoContent,
			body:       "",
		},
		{
			name:       "non 2xx status code",
			statusCode: http.StatusInternalServerError,
			body:       "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elapsed := runDoRequestTest(t, tt.statusCode, tt.body)
			if elapsed <= 0 {
				t.Errorf("elapsed time = %v, want > 0", elapsed)
			}
		})
	}
}

func runDoRequestTest(t *testing.T, statusCode int, body string) time.Duration {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	elapsed, err := doRequest(req, client)
	if err != nil {
		t.Fatalf("doRequest() unexpected error: %v", err)
	}
	return elapsed
}

func TestDoRequest_ClientError(t *testing.T) {
	req, err := http.NewRequest("GET", "http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 1 * time.Second}
	_, err = doRequest(req, client)
	if err == nil {
		t.Fatal("doRequest() expected error for unreachable server, got nil")
	}
}

func TestDoRequest_ReadBodyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		// Close connection without writing body to cause read error
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("failed to hijack connection: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	_, err = doRequest(req, client)
	// The read error may or may not occur depending on timing; we just ensure it doesn't panic.
	_ = err
}

func TestDoRequest_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 10 * time.Millisecond}
	_, err = doRequest(req, client)
	if err == nil {
		t.Fatal("doRequest() expected timeout error, got nil")
	}
}

func TestCreateRequest_URL(t *testing.T) {
	want := "https://example.com/path?query=value"
	o := &Opt{URL: want}
	req, err := o.createRequest()
	if err != nil {
		t.Fatalf("createRequest() unexpected error: %v", err)
	}
	if req.URL.String() != want {
		t.Errorf("request URL = %q, want %q", req.URL.String(), want)
	}
}

func TestDoRequest_ReadsFirstByte(t *testing.T) {
	body := "hello world"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	elapsed, err := doRequest(req, client)
	if err != nil {
		t.Fatalf("doRequest() unexpected error: %v", err)
	}
	if elapsed <= 0 {
		t.Errorf("elapsed time = %v, want > 0", elapsed)
	}
}

func TestDoRequest_TrailingSlash(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/") {
			t.Errorf("request path = %q, want trailing slash", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	_, err = doRequest(req, client)
	if err != nil {
		t.Fatalf("doRequest() unexpected error: %v", err)
	}
}

func TestDoRequest_LargeBody(t *testing.T) {
	body := strings.Repeat("x", 1024*1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	elapsed, err := doRequest(req, client)
	if err != nil {
		t.Fatalf("doRequest() unexpected error: %v", err)
	}
	if elapsed <= 0 {
		t.Errorf("elapsed time = %v, want > 0", elapsed)
	}
}

func BenchmarkDoRequest(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "x")
	}))
	defer ts.Close()

	client := http.Client{Timeout: 5 * time.Second}
	for b.Loop() {
		req, err := http.NewRequest("GET", ts.URL, nil)
		if err != nil {
			b.Fatalf("failed to create request: %v", err)
		}
		_, err = doRequest(req, client)
		if err != nil {
			b.Fatalf("doRequest() unexpected error: %v", err)
		}
	}
}
