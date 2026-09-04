package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testingT interface {
	Helper()
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
	Skip(args ...any)
}

// testServer creates a new httptest server that responds with the given status code and body.
func testServer(t testingT, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
}

// testServerWithHandler creates a new httptest server with the given handler.
func testServerWithHandler(t testingT, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

// newTestClient returns an http.Client with a timeout suitable for tests.
func newTestClient(timeout time.Duration) http.Client {
	return http.Client{Timeout: timeout}
}

// newGetRequest creates a new GET request for the given URL.
func newGetRequest(t testingT, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	return req
}

// assertElapsed asserts that the elapsed duration is positive.
func assertElapsed(t *testing.T, elapsed time.Duration) {
	t.Helper()
	assert.Positive(t, elapsed, "elapsed time should be positive")
}

// runGetStats runs getStats with the given options and returns the output.
func runGetStats(t *testing.T, o *Opt) string {
	t.Helper()
	var buf bytes.Buffer
	err := o.getStats(&buf)
	require.NoError(t, err)
	return buf.String()
}

// parseMetrics parses metric output into a map of name to value.
func parseMetrics(t *testing.T, output string) map[string]float64 {
	t.Helper()
	metrics := make(map[string]float64)
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.Split(line, "\t")
		require.Len(t, parts, 3, "invalid metric line: %q", line)
		value, err := strconv.ParseFloat(parts[1], 64)
		require.NoError(t, err, "failed to parse metric value %q", parts[1])
		metrics[parts[0]] = value
	}
	return metrics
}

// assertMetricContains asserts that the output contains the given metric substring.
func assertMetricContains(t *testing.T, output, metric string) {
	t.Helper()
	assert.Contains(t, output, metric)
}

// assertMetricNotContains asserts that the output does not contain the given metric substring.
func assertMetricNotContains(t *testing.T, output, metric string) {
	t.Helper()
	assert.NotContains(t, output, metric)
}

// assertMetricValue asserts that the metric value is approximately equal to the expected value.
func assertMetricValue(t *testing.T, metrics map[string]float64, name string, expected float64) {
	t.Helper()
	value, ok := metrics[name]
	require.True(t, ok, "metric %s not found", name)
	assert.InDelta(t, expected, value, 0.001, "metric %s value mismatch", name)
}
