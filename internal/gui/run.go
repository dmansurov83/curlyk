package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/jsonlex"
	"github.com/user/curlyk/internal/profiles"
	"github.com/user/curlyk/internal/runner"
)

// Request execution for the desktop window. The GUI never talks to the network
// itself: it assembles a request from the buffer with the same httpfile and
// runner packages the TUI uses, so both front ends send identical requests for
// identical .http source.

// requestAtCaret returns the request block the caret sits in, or nil.
func (u *ui) requestAtCaret() *httpfile.Request {
	reqs := httpfile.ParseFile(u.ed.Text())
	return httpfile.GetRequestAtLine(reqs, u.ed.doc.row+1)
}

// profileVars returns the @var map of the active profile (nil when none).
func (u *ui) profileVars() map[string]string {
	return profiles.Load(".", u.settings.ActiveProfile)
}

// substituteVars resolves every {{name}} / {{$fn}} placeholder of a request in
// place, taking values from the file's @var declarations merged with the active
// profile's (the file wins). The first unresolved placeholder aborts the run, so
// a raw "{{x}}" is never sent.
func (u *ui) substituteVars(req *httpfile.Request) error {
	vars := req.Vars
	if pv := u.profileVars(); len(pv) > 0 {
		vars = httpfile.MergeVars(pv, vars)
	}

	sub, err := httpfile.Substitute(req.URL, vars)
	if err != nil {
		return err
	}
	req.URL = sub
	for i := range req.Headers {
		hv, err := httpfile.Substitute(req.Headers[i].Value, vars)
		if err != nil {
			return err
		}
		req.Headers[i].Value = hv
	}
	if req.Body != "" {
		bv, err := httpfile.Substitute(req.Body, vars)
		if err != nil {
			return err
		}
		req.Body = bv
	}
	return nil
}

// substitutionMessage renders a substitution failure as the name of the first
// unresolved variable when the error carries one.
func substitutionMessage(err error) string {
	var se *httpfile.SubstitutionError
	if errors.As(err, &se) {
		return i18n.T("err.variable", se.First())
	}
	return i18n.T("err.variable", err.Error())
}

// runRequest executes the request under the caret on a background goroutine so
// the window stays responsive, and shows the outcome in the response pane. Its
// status messages are the console's, so both front ends report a run the same
// way.
func (u *ui) runRequest() {
	if u.running {
		return
	}
	req := u.requestAtCaret()
	if req == nil {
		u.setStatus(i18n.T("err.noRequestCursor"))
		return
	}
	if err := u.substituteVars(req); err != nil {
		u.setStatus(substitutionMessage(err))
		return
	}
	opts, err := runner.ApplyOptions(*req, runner.Options{})
	if err != nil {
		u.setStatus(i18n.T("err.option", err.Error()))
		return
	}

	u.setRunning(true)
	u.setStatus(i18n.T("status.running", req.Method, req.URL))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), opts.TimeoutOrDefault())
		defer cancel()

		res := runner.Run(ctx, *req, opts)
		body, _ := runner.BodyBytes(res.Response)
		text, headerRows := formatResponse(res, body)
		// Fyne widgets may only be touched from the main runtime context, so the
		// reply is handed over with fyne.Do instead of being written directly from
		// this goroutine.
		fyne.Do(func() {
			u.lastBody = body
			u.setResponse(text, headerRows)
			u.setRunning(false)

			switch {
			case res.Err != nil:
				u.setStatus(i18n.T("err.network", res.Err.Error()))
			case res.Response != nil:
				u.setStatus(fmt.Sprintf(i18n.T("resp.statusCode"), res.Response.StatusCode))
			}
		})
	}()
}

// formatResponse renders a run result as the text of the response pane: the
// request that was sent (method, resolved URL and headers), the status line, the
// timing, the response headers and the body. It also returns the number of lines
// that belong to the header part, so the pane can keep them neutral and highlight
// only the body.
func formatResponse(res *runner.Result, body []byte) (string, int) {
	var head strings.Builder

	if res.Err != nil {
		if res.Request != nil {
			head.WriteString(res.Request.Method + " " + res.Request.URL.String() + "\n")
		}
		head.WriteString(i18n.T("resp.netFailed") + "\n")
		if res.Duration > 0 {
			head.WriteString(i18n.T("resp.duration", res.Duration.Round(time.Millisecond).String()) + "\n")
		}
		head.WriteString("\n")
		return head.String() + res.Err.Error() + "\n", strings.Count(head.String(), "\n")
	}

	if res.Request != nil {
		head.WriteString(res.Request.Method + " " + res.Request.URL.String() + "\n")
		var reqHdr strings.Builder
		res.Request.Header.Write(&reqHdr)
		// http.Header.Write terminates lines with CRLF; the stray \r would rewind
		// the widget's own layout, so normalize to \n.
		head.WriteString(strings.ReplaceAll(reqHdr.String(), "\r\n", "\n"))
	}
	head.WriteString(fmt.Sprintf(i18n.T("resp.statusCode"), res.Response.StatusCode) +
		" (" + res.Status + ")\n")
	head.WriteString(i18n.T("resp.duration", res.Duration.Round(time.Millisecond).String()) + "\n")

	var hdr strings.Builder
	res.Response.Header.Write(&hdr)
	head.WriteString(strings.ReplaceAll(hdr.String(), "\r\n", "\n"))
	head.WriteString("\n")

	text := head.String()
	if len(body) > 0 {
		// A JSON body is shown pretty-printed, exactly like the console formats
		// response bodies (jsonlex.Pretty).
		text += jsonlex.Pretty(body)
		text += "\n"
	}
	return text, strings.Count(head.String(), "\n")
}

// copyResponse puts the reply on the clipboard — the "copy response" button of
// the pane. With a body it copies the raw body; a body-less reply (204, HEAD,
// network error) falls back to the formatted pane text, exactly as the console's
// copy button does. This keeps the button useful for every reply the pane can
// show.
func (u *ui) copyResponse() {
	if u.helpShown() {
		u.setStatus(i18n.T("err.noResponse"))
		return
	}
	text := string(u.lastBody)
	if text == "" {
		text = u.respText
	}
	if text == "" || text == i18n.T("gui.noResponse") {
		u.setStatus(i18n.T("err.noResponse"))
		return
	}
	cb := clipboard()
	if cb == nil {
		u.setStatus(i18n.T("err.copyGeneric", "clipboard unavailable"))
		return
	}
	cb.SetContent(text)
	if got := cb.Content(); got != text {
		// Fyne's Windows clipboard can refuse the write (another process holds
		// it); reporting it is better than a silent no-op.
		u.setStatus(i18n.T("err.copyGeneric", "clipboard write failed"))
		return
	}
	u.setStatus(i18n.T("status.respCopied", len([]rune(text))))
}
