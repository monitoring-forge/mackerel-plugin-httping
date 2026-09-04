package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOpt_GetStats_Success(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     3,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "httping.test_rtt_count.success") {
		t.Errorf("output missing success metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_count.error") {
		t.Errorf("output missing error metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_ms.max") {
		t.Errorf("output missing max metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_ms.min") {
		t.Errorf("output missing min metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_ms.average") {
		t.Errorf("output missing average metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_ms.90_percentile") {
		t.Errorf("output missing 90_percentile metric: %s", output)
	}

	// Count includes preflight + Count requests (when successful)
	if requestCount != o.Count+1 {
		t.Errorf("request count = %d, want %d", requestCount, o.Count+1)
	}
}

func TestOpt_GetStats_InvalidURL(t *testing.T) {
	var buf bytes.Buffer
	o := &Opt{
		URL:       "://invalid-url",
		Timeout:   5000,
		Interval:  0,
		Count:     3,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err == nil {
		t.Fatal("getStats() expected error for invalid URL, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "httping.test_rtt_count.success\t0.000000") {
		t.Errorf("output missing zero success metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_count.error\t3.000000") {
		t.Errorf("output missing error count metric: %s", output)
	}
}

func TestOpt_GetStats_AllFailed(t *testing.T) {
	var buf bytes.Buffer
	o := &Opt{
		URL:       "http://127.0.0.1:1",
		Timeout:   100,
		Interval:  0,
		Count:     3,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "httping.test_rtt_count.success\t0.000000") {
		t.Errorf("output missing zero success metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_count.error\t3.000000") {
		t.Errorf("output missing error count metric: %s", output)
	}
	// No RTT metrics should be output when all failed
	if strings.Contains(output, "httping.test_rtt_ms.max") {
		t.Errorf("output should not contain max metric when all failed: %s", output)
	}
}

func TestOpt_GetStats_CountZero(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     0,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "httping.test_rtt_count.success\t0.000000") {
		t.Errorf("output missing success metric: %s", output)
	}
	if !strings.Contains(output, "httping.test_rtt_count.error\t0.000000") {
		t.Errorf("output missing error metric: %s", output)
	}
	if requestCount != 1 {
		t.Errorf("request count = %d, want 1 (preflight only)", requestCount)
	}
}

func TestOpt_GetStats_PreflightFails(t *testing.T) {
	var buf bytes.Buffer
	o := &Opt{
		URL:       "http://127.0.0.1:1",
		Timeout:   100,
		Interval:  0,
		Count:     2,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 output lines, got %d: %s", len(lines), output)
	}
}

func TestOpt_GetStats_OutputFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "example",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 output lines, got %d", len(lines))
	}

	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Errorf("line does not have 3 tab-separated parts: %q", line)
			continue
		}
		if _, err := strconv.ParseFloat(parts[1], 64); err != nil {
			t.Errorf("metric value is not float: %q", parts[1])
		}
		if _, err := strconv.ParseUint(parts[2], 10, 64); err != nil {
			t.Errorf("timestamp is not uint64: %q", parts[2])
		}
	}
}

func TestOpt_GetStats_RedirectNotFollowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		}
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL + "/redirect",
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	// Redirect responses (3xx) are treated as success by doRequest because it reads the response body (empty for redirects).
	if !strings.Contains(output, "httping.test_rtt_count.success") {
		t.Errorf("output missing success metric: %s", output)
	}
	if strings.Contains(output, "httping.test_rtt_count.error\t1.000000") {
		t.Errorf("redirect response should not be counted as error: %s", output)
	}
}

func TestOpt_GetStats_Interval(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  50,
		Count:     2,
		KeyPrefix: "test",
	}

	start := time.Now()
	err := o.getStats(&buf)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	// 2 requests with 50ms interval between them
	minExpected := 50 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("elapsed time %v is less than minimum expected %v", elapsed, minExpected)
	}
}

func TestOpt_GetStats_DisableKeepalive(t *testing.T) {
	connectionCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectionCount++
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:              ts.URL,
		Timeout:          5000,
		Interval:         0,
		Count:            2,
		KeyPrefix:        "test",
		DisableKeepalive: true,
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "httping.test_rtt_count.success") {
		t.Errorf("output missing success metric: %s", output)
	}
	_ = connectionCount
}

func TestOpt_GetStats_StatsValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     5,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 output lines, got %d", len(lines))
	}

	for _, line := range lines[2:] {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Fatalf("line does not have 3 parts: %q", line)
		}
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			t.Fatalf("failed to parse metric value: %v", err)
		}
		if value < 0 {
			t.Errorf("metric value should be non-negative: %f", value)
		}
	}
}

func TestOpt_GetStats_StatsMinMaxAveragePercentile(t *testing.T) {
	requestNum := 0
	delays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestNum < len(delays) {
			time.Sleep(delays[requestNum])
		}
		requestNum++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	var buf bytes.Buffer
	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     5,
		KeyPrefix: "test",
	}

	err := o.getStats(&buf)
	if err != nil {
		t.Fatalf("getStats() unexpected error: %v", err)
	}

	metrics := parseMetrics(t, buf.String())

	if metrics["httping.test_rtt_count.success"] != 5.0 {
		t.Errorf("success count = %f, want 5", metrics["httping.test_rtt_count.success"])
	}
	if metrics["httping.test_rtt_count.error"] != 0.0 {
		t.Errorf("error count = %f, want 0", metrics["httping.test_rtt_count.error"])
	}

	min := metrics["httping.test_rtt_ms.min"]
	max := metrics["httping.test_rtt_ms.max"]
	avg := metrics["httping.test_rtt_ms.average"]
	p90 := metrics["httping.test_rtt_ms.90_percentile"]

	if min <= 0 {
		t.Errorf("min should be positive, got %f", min)
	}
	if max < min {
		t.Errorf("max (%f) should be >= min (%f)", max, min)
	}
	if avg < min || avg > max {
		t.Errorf("average (%f) should be between min (%f) and max (%f)", avg, min, max)
	}
	if p90 < min || p90 > max {
		t.Errorf("90th percentile (%f) should be between min (%f) and max (%f)", p90, min, max)
	}

	expectedAvg := 30.0
	if avg < expectedAvg*0.5 || avg > expectedAvg*2.0 {
		t.Errorf("average (%f) is far from expected ~%f", avg, expectedAvg)
	}
	expectedP90 := 46.0
	if p90 < expectedP90*0.5 || p90 > expectedP90*2.0 {
		t.Errorf("90th percentile (%f) is far from expected ~%f", p90, expectedP90)
	}
}

func parseMetrics(t *testing.T, output string) map[string]float64 {
	t.Helper()
	metrics := make(map[string]float64)
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Fatalf("invalid metric line: %q", line)
		}
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			t.Fatalf("failed to parse metric value %q: %v", parts[1], err)
		}
		metrics[parts[0]] = value
	}
	return metrics
}

func TestOpt_Run(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "test",
	}

	result, code := o.Run(nil)
	if result != nil {
		t.Errorf("Run() result = %v, want nil", result)
	}
	if code != 0 {
		t.Errorf("Run() code = %d, want 0", code)
	}
}

func TestOpt_Run_WithError(t *testing.T) {
	o := &Opt{
		URL:       "://invalid-url",
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "test",
	}

	result, code := o.Run(nil)
	if result == nil {
		t.Fatal("Run() expected non-nil result for error")
	}
	if code != 2 {
		t.Errorf("Run() code = %d, want 2", code)
	}
}

func BenchmarkGetStats(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	for b.Loop() {
		var buf bytes.Buffer
		o := &Opt{
			URL:       ts.URL,
			Timeout:   5000,
			Interval:  0,
			Count:     1,
			KeyPrefix: "bench",
		}
		if err := o.getStats(&buf); err != nil {
			b.Fatalf("getStats() unexpected error: %v", err)
		}
	}
}
