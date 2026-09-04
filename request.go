package main

import (
	"errors"
	"io"
	"net/http"
	"time"
)

func (o *Opt) createRequest() (*http.Request, error) {
	return http.NewRequest("GET", o.URL, nil)
}

func doRequest(req *http.Request, client http.Client) (time.Duration, error) {
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	oneByte := make([]byte, 1)
	_, err = resp.Body.Read(oneByte)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	// Use Start Transfer timing
	elapsed := time.Since(start)
	return elapsed, nil
}
