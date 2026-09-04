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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOpt(ts *httptest.Server, count int) *Opt {
	return &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     count,
		KeyPrefix: "test",
	}
}

func TestOpt_GetStats_Success(t *testing.T) {
	requestCount := 0
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	o := newTestOpt(ts, 3)
	output := runGetStats(t, o)

	assertMetricContains(t, output, "httping.test_rtt_count.success")
	assertMetricContains(t, output, "httping.test_rtt_count.error")
	assertMetricContains(t, output, "httping.test_rtt_ms.max")
	assertMetricContains(t, output, "httping.test_rtt_ms.min")
	assertMetricContains(t, output, "httping.test_rtt_ms.average")
	assertMetricContains(t, output, "httping.test_rtt_ms.90_percentile")
	assert.Equal(t, o.Count+1, requestCount)
}

func TestOpt_GetStats_InvalidURL(t *testing.T) {
	o := &Opt{
		URL:       "://invalid-url",
		Timeout:   5000,
		Interval:  0,
		Count:     3,
		KeyPrefix: "test",
	}

	var buf bytes.Buffer
	err := o.getStats(&buf)
	require.Error(t, err)

	output := buf.String()
	assertMetricContains(t, output, "httping.test_rtt_count.success\t0.000000")
	assertMetricContains(t, output, "httping.test_rtt_count.error\t3.000000")
}

func TestOpt_GetStats_AllFailed(t *testing.T) {
	o := &Opt{
		URL:       "http://127.0.0.1:1",
		Timeout:   100,
		Interval:  0,
		Count:     3,
		KeyPrefix: "test",
	}

	output := runGetStats(t, o)
	assertMetricContains(t, output, "httping.test_rtt_count.success\t0.000000")
	assertMetricContains(t, output, "httping.test_rtt_count.error\t3.000000")
	assertMetricNotContains(t, output, "httping.test_rtt_ms.max")
}

func TestOpt_GetStats_CountZero(t *testing.T) {
	requestCount := 0
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	o := newTestOpt(ts, 0)
	output := runGetStats(t, o)

	assertMetricContains(t, output, "httping.test_rtt_count.success\t0.000000")
	assertMetricContains(t, output, "httping.test_rtt_count.error\t0.000000")
	assert.Equal(t, 1, requestCount)
}

func TestOpt_GetStats_PreflightFails(t *testing.T) {
	o := &Opt{
		URL:       "http://127.0.0.1:1",
		Timeout:   100,
		Interval:  0,
		Count:     2,
		KeyPrefix: "test",
	}

	output := runGetStats(t, o)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2)
}

func TestOpt_GetStats_OutputFormat(t *testing.T) {
	ts := testServer(t, http.StatusOK, "ok")
	defer ts.Close()

	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "example",
	}
	output := runGetStats(t, o)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 6)
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		require.Len(t, parts, 3, "line does not have 3 tab-separated parts: %q", line)
		_, err := strconv.ParseFloat(parts[1], 64)
		require.NoError(t, err, "metric value is not float: %q", parts[1])
		_, err = strconv.ParseUint(parts[2], 10, 64)
		require.NoError(t, err, "timestamp is not uint64: %q", parts[2])
	}
}

func TestOpt_GetStats_RedirectNotFollowed(t *testing.T) {
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		}
	})
	defer ts.Close()

	o := &Opt{
		URL:       ts.URL + "/redirect",
		Timeout:   5000,
		Interval:  0,
		Count:     1,
		KeyPrefix: "test",
	}
	output := runGetStats(t, o)

	assertMetricContains(t, output, "httping.test_rtt_count.success")
	assertMetricNotContains(t, output, "httping.test_rtt_count.error\t1.000000")
}

func TestOpt_GetStats_Interval(t *testing.T) {
	ts := testServer(t, http.StatusOK, "ok")
	defer ts.Close()

	o := &Opt{
		URL:       ts.URL,
		Timeout:   5000,
		Interval:  50,
		Count:     2,
		KeyPrefix: "test",
	}

	start := time.Now()
	output := runGetStats(t, o)
	elapsed := time.Since(start)

	_ = output
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestOpt_GetStats_DisableKeepalive(t *testing.T) {
	connectionCount := 0
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		connectionCount++
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	o := &Opt{
		URL:              ts.URL,
		Timeout:          5000,
		Interval:         0,
		Count:            2,
		KeyPrefix:        "test",
		DisableKeepalive: true,
	}
	output := runGetStats(t, o)

	assertMetricContains(t, output, "httping.test_rtt_count.success")
	_ = connectionCount
}

func TestOpt_GetStats_StatsValues(t *testing.T) {
	ts := testServer(t, http.StatusOK, "ok")
	defer ts.Close()

	o := newTestOpt(ts, 5)
	output := runGetStats(t, o)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 6)
	for _, line := range lines[2:] {
		parts := strings.Split(line, "\t")
		require.Len(t, parts, 3, "line does not have 3 parts: %q", line)
		value, err := strconv.ParseFloat(parts[1], 64)
		require.NoError(t, err, "failed to parse metric value: %v", err)
		assert.GreaterOrEqual(t, value, 0.0)
	}
}

func TestOpt_GetStats_StatsMinMaxAveragePercentile(t *testing.T) {
	requestNum := 0
	delays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond}
	ts := testServerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if requestNum < len(delays) {
			time.Sleep(delays[requestNum])
		}
		requestNum++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	defer ts.Close()

	o := newTestOpt(ts, 5)
	metrics := parseMetrics(t, runGetStats(t, o))

	assertMetricValue(t, metrics, "httping.test_rtt_count.success", 5.0)
	assertMetricValue(t, metrics, "httping.test_rtt_count.error", 0.0)

	min := metrics["httping.test_rtt_ms.min"]
	max := metrics["httping.test_rtt_ms.max"]
	avg := metrics["httping.test_rtt_ms.average"]
	p90 := metrics["httping.test_rtt_ms.90_percentile"]

	assert.Positive(t, min, "min should be positive")
	assert.GreaterOrEqual(t, max, min)
	assert.GreaterOrEqual(t, avg, min)
	assert.LessOrEqual(t, avg, max)
	assert.GreaterOrEqual(t, p90, min)
	assert.LessOrEqual(t, p90, max)
	assert.InDelta(t, 30.0, avg, 15.0, "average is far from expected")
	assert.InDelta(t, 46.0, p90, 23.0, "90th percentile is far from expected")
}

func TestOpt_Run(t *testing.T) {
	ts := testServer(t, http.StatusOK, "ok")
	defer ts.Close()

	o := newTestOpt(ts, 1)
	result, code := o.Run(nil)
	assert.Nil(t, result)
	assert.Equal(t, 0, code)
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
	assert.NotNil(t, result)
	assert.Equal(t, 2, code)
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
