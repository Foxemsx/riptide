package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"
)

const (
	fallbackToken   = "YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm"
	cloudflareUpURL = "https://speed.cloudflare.com/__up"
	uploadChunkSize = 1024 * 1024 // 1MB chunks
	uploadConns     = 6
)

var (
	scriptExpr = regexp.MustCompile(`app-[a-z0-9]+\.js`)
	tokenExpr  = regexp.MustCompile(`token:"(\w+)"`)
)

func fetchToken() string {
	page, err := httpGet("https://fast.com/")
	if err != nil {
		return fallbackToken
	}

	script, err := httpGet("https://fast.com/" + scriptExpr.FindString(string(page)))
	if err != nil {
		return fallbackToken
	}

	match := tokenExpr.FindSubmatch(script)
	if len(match) < 2 {
		return fallbackToken
	}
	return string(match[1])
}

type targetResponse struct {
	Targets []struct {
		URL string `json:"url"`
	} `json:"targets"`
}

func fetchTargets(count int) ([]string, error) {
	url := fmt.Sprintf(
		"https://api.fast.com/netflix/speedtest/v2?https=true&token=%s&urlCount=%d",
		fetchToken(), count,
	)
	body, err := httpGet(url)
	if err != nil {
		return nil, err
	}

	var resp targetResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	urls := make([]string, len(resp.Targets))
	for i, t := range resp.Targets {
		urls[i] = t.URL
	}
	return urls, nil
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

type byteCounter struct {
	total *atomic.Int64
}

func (c byteCounter) Write(p []byte) (int, error) {
	c.total.Add(int64(len(p)))
	return len(p), nil
}

func download(ctx context.Context, url string, total *atomic.Int64) {
	for ctx.Err() == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		io.Copy(byteCounter{total}, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}
}

func upload(ctx context.Context, _ string, total *atomic.Int64) {
	data := make([]byte, uploadChunkSize)
	rand.Read(data) //nolint:errcheck

	tr := &http.Transport{
		MaxIdleConns:        1,
		MaxConnsPerHost:     1,
		MaxIdleConnsPerHost: 1,
		DisableCompression:  true,
	}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	for ctx.Err() == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudflareUpURL, bytes.NewReader(data))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

		resp, err := client.Do(req)
		if err != nil {
			return
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		total.Add(int64(len(data)))
	}
}

func mbps(bytes int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(bytes) * 8 / d.Seconds() / 1e6
}

func scale(speed float64) (float64, string) {
	if speed >= 999.95 {
		return speed / 1000, "Gbps"
	}
	return speed, "Mbps"
}

func checkNetwork() error {
	_, err := net.LookupHost("fast.com")
	if err != nil {
		return errors.New("no internet connection")
	}
	return nil
}
