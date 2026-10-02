package runner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/user/curlyk/internal/httpfile"
)

// defaultTimeout is applied when Options.Timeout is zero.
const defaultTimeout = 30 * time.Second

// shared transports keep their HTTP keep-alive connection pools alive between
// Run calls. Connection reuse lives in *http.Transport, so the transports are
// shared while a fresh, stateless *http.Client is built per request.
//
// A Transport's TLSClientConfig can only be set before first use, so the two
// variants (secure / insecure) are created once here and never mutated later.
var (
	// secureTransport is http.DefaultTransport itself, chosen for its extended
	// keep-alive defaults. The pool it maintains is preserved across runs.
	secureTransport http.RoundTripper = http.DefaultTransport

	// insecureTransport mirrors DefaultTransport but disables TLS verification.
	// Its TLS config is fixed here, before any use, so it is safe to share.
	insecureTransport = func() http.RoundTripper {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- user-requested
		return tr
	}()
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

// DefaultTimeout returns the timeout applied when Options.Timeout is zero.
func DefaultTimeout() time.Duration { return defaultTimeout }

// TimeoutOrDefault normalizes opts.Timeout: a zero timeout becomes defaultTimeout.
func (o Options) TimeoutOrDefault() time.Duration {
	if o.Timeout == 0 {
		return defaultTimeout
	}
	return o.Timeout
}

// ApplyOptions folds the processing options parsed from a .http request into a
// copy of opts. File options override the passed defaults; options whose value
// cannot be parsed leave the corresponding field untouched and are reported in
// the returned error. The returned Options is safe to pass straight to Run.
func ApplyOptions(req httpfile.Request, opts Options) (Options, error) {
	var err error
	for _, opt := range req.Options {
		switch opt.Name {
		case "no-redirect", "noredirect":
			opts.FollowRedirects = false
		case "follow-redirects", "followredirects", "redirects":
			if v := strings.TrimSpace(strings.ToLower(opt.Value)); v == "on" || v == "true" || v == "yes" {
				opts.FollowRedirects = true
			} else if v == "off" || v == "false" || v == "no" {
				opts.FollowRedirects = false
			}
		case "insecure":
			opts.Insecure = true
		case "timeout":
			if d, perr := parseTimeout(opt.Value); perr == nil && d > 0 {
				opts.Timeout = d
			} else if perr != nil {
				err = perr
			}
		}
	}
	return opts, err
}

// parseTimeout parses a timeout option value: a bare number is seconds ("5"),
// an optional unit may follow ("500ms", "5s", "2m"). White space is trimmed.
func parseTimeout(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty timeout")
	}
	// A bare integer treats as seconds (JetBrains convention).
	if n, aerr := strconv.Atoi(s); aerr == nil {
		return time.Duration(n) * time.Second, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	return 0, fmt.Errorf("invalid timeout %q", s)
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
	clientTimeout := opts.TimeoutOrDefault()

	// Pick the shared transport matching the TLS requirement; its pool is kept
	// between runs. Timeout and redirect handling vary per request, so a fresh
	// Client is assembled here — it holds no mutable state that could collide
	// with concurrent runs on the same transport.
	transport := secureTransport
	if opts.Insecure {
		transport = insecureTransport
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   clientTimeout,
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
