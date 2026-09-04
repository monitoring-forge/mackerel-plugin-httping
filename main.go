package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/monitoring-forge/flagrun"
	"github.com/montanaflynn/stats"
)

var version string

type Opt struct {
	URL              string `long:"url" description:"URL to ping" required:"true"`
	Timeout          int    `long:"timeout" default:"5000" description:"timeout millisec per ping"`
	Interval         int    `long:"interval" default:"200" description:"sleep millisec after every ping"`
	Count            int    `long:"count" default:"10" description:"Count Sending ping"`
	KeyPrefix        string `long:"key-prefix" description:"Metric key prefix" required:"true"`
	DisableKeepalive bool   `long:"disable-keepalive" description:"disable keepalive"`
	Version          bool   `short:"v" long:"version" description:"Show version"`
}

func (o *Opt) createClient() http.Client {
	resolver := &net.Resolver{}
	ips := make([]net.IPAddr, 0)

	baseDialer := (&net.Dialer{
		Timeout:   time.Millisecond * time.Duration(o.Timeout),
		KeepAlive: 30 * time.Second,
	}).DialContext

	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		h, p, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		if len(ips) == 0 {
			ips, err = resolver.LookupIPAddr(ctx, h)
			if err != nil {
				return nil, err
			}
		}

		return baseDialer(ctx, "tcp", net.JoinHostPort(ips[rand.Intn(len(ips))].String(), p))
	}

	client := http.Client{
		Timeout: time.Millisecond * time.Duration(o.Timeout),
		Transport: &http.Transport{
			DialContext:           dialer,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: time.Millisecond * time.Duration(o.Timeout),
			DisableKeepAlives:     o.DisableKeepalive,
		},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client

}

func (o *Opt) getStats(w io.Writer) error {

	var rtts []float64
	suceeded := 0.0
	failed := 0.0
	t := float64(0)

	// preflight
	preReq, err := o.createRequest()
	if err != nil {
		errorNow := uint64(time.Now().Unix())
		fmt.Fprintf(w, "httping.%s_rtt_count.success\t%f\t%d\n", o.KeyPrefix, 0.0, errorNow)
		fmt.Fprintf(w, "httping.%s_rtt_count.error\t%f\t%d\n", o.KeyPrefix, float64(o.Count), errorNow)
		return err
	}
	client := o.createClient()
	_, err = doRequest(preReq, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error in preflight: %v\n", err)
	}

	for range o.Count {
		time.Sleep(time.Millisecond * time.Duration(o.Interval))
		req, _ := o.createRequest()
		elapsed, err := doRequest(req, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in request: %v\n", err)
			failed++
			continue
		}

		rttMilliSec := float64(elapsed.Nanoseconds()) / 1000.0 / 1000.0
		rtts = append(rtts, rttMilliSec)
		t += rttMilliSec
		suceeded++
	}

	now := uint64(time.Now().Unix())
	fmt.Fprintf(w, "httping.%s_rtt_count.success\t%f\t%d\n", o.KeyPrefix, suceeded, now)
	fmt.Fprintf(w, "httping.%s_rtt_count.error\t%f\t%d\n", o.KeyPrefix, failed, now)
	if suceeded > 0 {
		max, err := stats.Max(rtts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in calculating max: %v\n", err)
		}
		min, err := stats.Min(rtts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in calculating min: %v\n", err)
		}
		mean, err := stats.Mean(rtts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in calculating average: %v\n", err)
		}
		percentile90, err := stats.Percentile(rtts, 90)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in calculating 90th percentile: %v\n", err)
		}
		fmt.Fprintf(w, "httping.%s_rtt_ms.max\t%f\t%d\n", o.KeyPrefix, max, now)
		fmt.Fprintf(w, "httping.%s_rtt_ms.min\t%f\t%d\n", o.KeyPrefix, min, now)
		fmt.Fprintf(w, "httping.%s_rtt_ms.average\t%f\t%d\n", o.KeyPrefix, mean, now)
		fmt.Fprintf(w, "httping.%s_rtt_ms.90_percentile\t%f\t%d\n", o.KeyPrefix, percentile90, now)
	}

	return nil
}

func (o *Opt) Run(_ []string) (any, int) {
	err := o.getStats(os.Stdout)
	if err != nil {
		return err, flagrun.CRITICAL
	}
	return nil, flagrun.OK
}

func main() {
	os.Exit(flagrun.Go(&Opt{}, flagrun.Version(version)))
}
