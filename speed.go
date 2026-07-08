package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// --- Configuration -------------------------------------------------------

const (
	// fast.com config endpoint. The token is the public app token shipped in
	// fast.com's web bundle; it is required by the API and is not secret.
	configURL = "https://api.fast.com/netflix/speedtest/v2?https=true&token=YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm&urlCount=%d"

	// Number of parallel connections used to saturate the link.
	defaultConnections = 5
	// Duration of each transfer phase (download / upload).
	defaultDuration = 10 * time.Second
	// Size of a single upload POST body. We reuse the same random buffer
	// repeatedly, mirroring fast.com's POST-to-target approach.
	uploadChunkSize = 256 * 1024
)

// --- Data structures -----------------------------------------------------

type target struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type configResponse struct {
	Client  clientInfo `json:"client"`
	Targets []target   `json:"targets"`
}

type clientInfo struct {
	IP       string `json:"ip"`
	ASN      string `json:"asn"`
	Location struct {
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"location"`
}

// Sample is one instantaneous speed measurement broadcast to the UI each tick.
type Sample struct {
	Phase Phase   // which phase produced this sample
	Bytes uint64  // bytes transferred so far in the current phase
	Rate  float64 // instantaneous rate in bytes/sec
	At    time.Time
}

// Progress is the live channel payload the UI consumes.
type Progress struct {
	URLs       []string   // discovered target URLs (set once)
	ServerName string     // human-readable server/region label (set once)
	Phases     chan Phase // phase transitions
	Samples    chan Sample
	Result     chan Result
	Err        error
}

// Phase enumerates the test lifecycle.
type Phase int

const (
	PhaseInit Phase = iota
	PhaseFinding
	PhaseConnected
	PhaseDownload
	PhaseUpload
	PhaseLatency
	PhaseDone
)

func (p Phase) String() string {
	switch p {
	case PhaseFinding:
		return "finding servers"
	case PhaseConnected:
		return "connected"
	case PhaseDownload:
		return "download"
	case PhaseUpload:
		return "upload"
	case PhaseLatency:
		return "measuring latency"
	case PhaseDone:
		return "done"
	default:
		return "starting"
	}
}

// Result is the final measurement summary.
type Result struct {
	DownloadMbps float64
	UploadMbps   float64
	PingMs       float64
	DownloadPeak float64
	UploadPeak   float64
	Client       string // human-readable location/ISP if known
}

// --- Target discovery ----------------------------------------------------

func fetchTargets(connections int) ([]string, []string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf(configURL, connections))
	if err != nil {
		return nil, nil, "", fmt.Errorf("could not reach speed-test servers: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, "", fmt.Errorf("speed-test server returned status %d", resp.StatusCode)
	}
	var cr configResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, nil, "", fmt.Errorf("could not parse server list: %w", err)
	}
	if len(cr.Targets) == 0 {
		return nil, nil, "", fmt.Errorf("no speed-test servers were found")
	}
	urls := make([]string, len(cr.Targets))
	names := make([]string, len(cr.Targets))
	for i, t := range cr.Targets {
		urls[i] = t.URL
		names[i] = t.Name
	}
	// Build a friendly region label from the client location.
	loc := cr.Client.Location
	region := strings.TrimSpace(loc.City)
	if loc.Country != "" {
		if region != "" {
			region += ", "
		}
		region += loc.Country
	}
	return urls, names, region, nil
}

// --- Transfer engine -----------------------------------------------------

// counter is an atomic byte counter shared across parallel connections.
type counter struct {
	n uint64
}

func (c *counter) add(b int)     { atomic.AddUint64(&c.n, uint64(b)) }
func (c *counter) value() uint64 { return atomic.LoadUint64(&c.n) }

// runPhase opens `conn` parallel connections to the given URLs and streams
// data for `duration`, counting bytes into `c`. It returns when the duration
// elapses or the context is cancelled. After a short settle delay it stops
// opening new connections so in-flight transfers finish cleanly.
func runPhase(ctx context.Context, urls []string, duration time.Duration, c *counter, doPost bool) {
	if len(urls) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	// Distribute connections round-robin across the discovered URLs.
	for i := 0; i < defaultConnections; i++ {
		url := urls[i%len(urls)]
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if doPost {
					postOnce(ctx, url, c)
				} else {
					getOnce(ctx, url, c)
				}
			}
		}(url)
	}
	wg.Wait()
}

// httpClient is used for transfers. It inherits ctx cancellation (via
// NewRequestWithContext) and has a request timeout so a stalled connection
// cannot hang wg.Wait() past the phase duration.
var httpClient = &http.Client{Timeout: 15 * time.Second}

func getOnce(ctx context.Context, url string, c *counter) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	// Wait for the connection to either succeed or be cancelled.
	type respErr struct {
		resp *http.Response
		err  error
	}
	ch := make(chan respErr, 1)
	go func() {
		resp, e := httpClient.Do(req)
		ch <- respErr{resp, e}
	}()
	select {
	case <-ctx.Done():
		return
	case re := <-ch:
		if re.err != nil || re.resp == nil {
			return
		}
		resp := re.resp
		defer resp.Body.Close()
		buf := make([]byte, 32*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := resp.Body.Read(buf)
			if n > 0 {
				c.add(n)
			}
			if err != nil {
				return
			}
		}
	}
}

func postOnce(ctx context.Context, url string, c *counter) {
	body := make([]byte, uploadChunkSize)
	rand.Read(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, io.NopCloser(newBytesReader(body)))
	if err != nil {
		return
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	ch := make(chan error, 1)
	go func() {
		resp, e := httpClient.Do(req)
		if e != nil {
			ch <- e
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		ch <- nil
	}()
	select {
	case <-ctx.Done():
		return
	case e := <-ch:
		if e != nil {
			return
		}
	}
	c.add(len(body))
}

// bytesReader is a small helper so we can reuse a single buffer as the POST
// body while still counting each send.
type bytesReader struct {
	b   []byte
	pos int
}

func newBytesReader(b []byte) *bytesReader { return &bytesReader{b: b} }

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

// --- Latency probe -------------------------------------------------------

// measureLatency does a single round-trip GET to one target and returns the
// round-trip time in milliseconds.
func measureLatency(ctx context.Context, url string) (float64, error) {
	if url == "" {
		return 0, fmt.Errorf("no target for latency probe")
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, nil
}

// --- Orchestration -------------------------------------------------------

// Run executes the full test: discover targets, download, upload, latency.
// It streams Phase transitions on p.Phases, Samples on p.Samples, and a final
// Result on p.Result. The caller is expected to cancel the passed context to
// abort (e.g. on quit).
func Run(ctx context.Context, p *Progress, connections int, duration time.Duration) {
	if connections <= 0 {
		connections = defaultConnections
	}
	if duration <= 0 {
		duration = defaultDuration
	}

	// Phase: finding servers.
	sendPhase(p, PhaseFinding)
	urls, names, region, err := fetchTargets(connections)
	if err != nil {
		p.Err = err
		p.Result <- Result{}
		return
	}
	p.URLs = urls
	if region != "" {
		p.ServerName = region
	} else if len(names) > 0 {
		p.ServerName = names[0]
	}

	// Phase: connected — hold a brief, explicit beat so the transition from
	// "finding servers" to live numbers reads as deliberate, not instant.
	sendPhase(p, PhaseConnected)
	select {
	case <-time.After(900 * time.Millisecond):
	case <-ctx.Done():
		p.Result <- Result{}
		return
	}

	// Phase: download.
	sendPhase(p, PhaseDownload)
	dlCounter := &counter{}
	dlPeak := runTimedPhase(ctx, urls, duration, dlCounter, false, p, PhaseDownload)
	dlBytes := dlCounter.value()

	// Phase: upload.
	sendPhase(p, PhaseUpload)
	ulCounter := &counter{}
	ulPeak := runTimedPhase(ctx, urls, duration, ulCounter, true, p, PhaseUpload)
	ulBytes := ulCounter.value()

	// Phase: latency.
	sendPhase(p, PhaseLatency)
	ping, err := measureLatency(ctx, urls[0])
	if err != nil {
		// Non-fatal: we can still report speed without a ping.
		ping = 0
	}

	sendPhase(p, PhaseDone)
	p.Result <- Result{
		DownloadMbps: bytesToMbps(dlBytes, duration),
		UploadMbps:   bytesToMbps(ulBytes, duration),
		PingMs:       ping,
		DownloadPeak: dlPeak,
		UploadPeak:   ulPeak,
	}
}

// momentarily not draining (or the buffer is full) we drop the message rather
// than block the engine — the model's phase watchdog covers any missed
// transition, so the UI can never stall on a blocked send here.
func sendPhase(p *Progress, ph Phase) {
	select {
	case p.Phases <- ph:
	default:
	}
}

// sendSample delivers one instantaneous speed sample to the live UI bridge.
// Like sendPhase it is non-blocking: if the model is momentarily not draining
// the channel we drop the sample rather than stall the engine (the next tick
// covers it).
func sendSample(p *Progress, s Sample) bool {
	select {
	case p.Samples <- s:
		return true
	default:
		return false
	}
}

// runTimedPhase opens parallel connections for `duration`, counting bytes into
// c. Samples are forwarded to p.Samples (for live UI) AND collected locally to
// compute the peak rate. The sampler is stopped via a child context before the
// local channel is drained and closed, avoiding a send-on-closed-channel race.
func runTimedPhase(ctx context.Context, urls []string, duration time.Duration, c *counter, doPost bool, p *Progress, ph Phase) float64 {
	local := make(chan Sample, 128)
	sctx, cancel := context.WithCancel(ctx)
	peakDone := make(chan float64, 1)
	go func() {
		var peak float64
		for s := range local {
			if mbps := bytesPerSecToMbps(s.Rate); mbps > peak {
				peak = mbps
			}
		}
		peakDone <- peak
	}()
	go sampleLoop(sctx, c, local, p.Samples, ph)
	runPhase(ctx, urls, duration, c, doPost)
	cancel() // stop the sampler goroutine

	select {
	case peak := <-peakDone:
		return peak
	case <-time.After(duration + 5*time.Second):
		return 0
	}
}

// sampleLoop periodically snapshots the counter and emits a Sample tagged with
// the active phase. Each sample is sent to both out (local peak math) and
// live (the UI bridge); both sends respect ctx cancellation.
func sampleLoop(ctx context.Context, c *counter, out chan<- Sample, live chan<- Sample, ph Phase) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer close(out)
	var last uint64
	var lastT = time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			cur := c.value()
			dt := now.Sub(lastT).Seconds()
			var rate float64
			if dt > 0 {
				rate = float64(cur-last) / dt
			}
			last, lastT = cur, now
			s := Sample{Phase: ph, Bytes: cur, Rate: rate, At: now}
			// Forward to the local collector (peak math) and the live UI
			// bridge. Either send may be blocked/closed; ctx cancellation
			// takes priority so we never leak.
			select {
			case out <- s:
			case <-ctx.Done():
				return
			}
			select {
			case live <- s:
			default:
			}
		}
	}
}

// --- Unit helpers --------------------------------------------------------

func bytesToMbps(bytes uint64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return bytesPerSecToMbps(float64(bytes) / d.Seconds())
}

func bytesPerSecToMbps(bps float64) float64 {
	const bitsPerByte = 8
	const mega = 1_000_000
	return (bps * bitsPerByte) / mega
}
