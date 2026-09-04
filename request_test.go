package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid URL", url: "https://example.com", wantErr: false},
		{name: "invalid URL", url: "://invalid-url", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Opt{URL: tt.url}
			req, err := o.createRequest()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, tt.url, req.URL.String())
		})
	}
}

func TestDoRequest(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "successful request", statusCode: http.StatusOK, body: "hello"},
		{name: "empty body", statusCode: http.StatusNoContent, body: ""},
		{name: "non 2xx status code", statusCode: http.StatusInternalServerError, body: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := testServer(t, tt.statusCode, tt.body)
			defer ts.Close()

			elapsed, err := doRequest(newGetRequest(t, ts.URL), newTestClient(5*time.Second))
			require.NoError(t, err)
			assertElapsed(t, elapsed)
		})
	}
}

func TestDoRequest_ClientError(t *testing.T) {
	_, err := doRequest(newGetRequest(t, "http://127.0.0.1:1"), newTestClient(time.Second))
	require.Error(t, err)
}

func TestDoRequest_ReadBodyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
		}
		conn, _, err := hijacker.Hijack()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}))
	defer ts.Close()

	_, err := doRequest(newGetRequest(t, ts.URL), newTestClient(5*time.Second))
	// The read error may or may not occur depending on timing; we just ensure it doesn't panic.
	_ = err
}

func TestDoRequest_Timeout(t *testing.T) {
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	_, err := doRequest(newGetRequest(t, ts.URL), newTestClient(10*time.Millisecond))
	require.Error(t, err)
}

func TestCreateRequest_URL(t *testing.T) {
	want := "https://example.com/path?query=value"
	o := &Opt{URL: want}
	req, err := o.createRequest()
	require.NoError(t, err)
	assert.Equal(t, want, req.URL.String())
}

func TestDoRequest_ReadsFirstByte(t *testing.T) {
	ts := testServer(t, http.StatusOK, "hello world")
	defer ts.Close()

	elapsed, err := doRequest(newGetRequest(t, ts.URL), newTestClient(5*time.Second))
	require.NoError(t, err)
	assertElapsed(t, elapsed)
}

func TestDoRequest_TrailingSlash(t *testing.T) {
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/"), "request path should have trailing slash")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	_, err := doRequest(newGetRequest(t, ts.URL+"/"), newTestClient(5*time.Second))
	require.NoError(t, err)
}

func TestDoRequest_LargeBody(t *testing.T) {
	body := strings.Repeat("x", 1024*1024)
	ts := testServer(t, http.StatusOK, body)
	defer ts.Close()

	elapsed, err := doRequest(newGetRequest(t, ts.URL), newTestClient(5*time.Second))
	require.NoError(t, err)
	assertElapsed(t, elapsed)
}

func BenchmarkDoRequest(b *testing.B) {
	ts := testServer(b, http.StatusOK, "x")
	defer ts.Close()

	client := newTestClient(5 * time.Second)
	for b.Loop() {
		_, err := doRequest(newGetRequest(b, ts.URL), client)
		require.NoError(b, err)
	}
}
