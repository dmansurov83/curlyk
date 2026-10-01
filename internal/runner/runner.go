package runner

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/user/curlyk/internal/httpfile"
)

// Options controls how requests are executed.
type Options struct {
	// FollowRedirects follows 3xx redirects (default false, like JetBrains).
	FollowRedirects bool
	// Timeout is the per-request timeout (default 30s).
	Timeout time.Duration
	// Insecure disables TLS verification.
	Insecure bool
}

// Result carries the executed request details and raw response.
type Result struct {
	Request  *http.Request
	Response *http.Response
	// Status is e.g. "200 OK".
	Status string
	// Duration of the request round trip.
	Duration time.Duration
	// Error is set when the request could not be completed.
	Err error
}

// Run executes a parsed .http request block.
func Run(ctx context.Context, req httpfile.Request, opts Options) *Result {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.Insecure}, // #nosec G402 -- user-requested
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   opts.Timeout,
	}
	if !opts.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return &Result{Err: err}
	}

	for _, h := range req.Headers {
		if h.Name == "" {
			continue
		}
		// Host header requires special handling.
		if strings.EqualFold(h.Name, "Host") {
			httpReq.Host = h.Value
			continue
		}
		httpReq.Header.Set(h.Name, h.Value)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	dur := time.Since(start)
	if err != nil {
		return &Result{Request: httpReq, Duration: dur, Err: err}
	}

	return &Result{
		Request:  httpReq,
		Response: resp,
		Status:   resp.Status,
		Duration: dur,
	}
}

// BodyBytes reads the full response body (and closes it).
func BodyBytes(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}