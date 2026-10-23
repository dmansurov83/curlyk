package completion

import (
	"net/url"
	"sort"

	"github.com/user/curlyk/internal/httpfile"
)

// ContextForSource collects the candidate words for autocomplete from .http
// source and the variables of the active profile: header names seen in the file,
// hosts from request URLs and @var values, the file's and the profile's variable
// names, the HTTP methods and the built-in {{$fn}} functions.
//
// It is the single collector used by both front ends, so the TUI and the desktop
// window suggest the same things.
func ContextForSource(src string, profileVars map[string]string) Context {
	ctx := Context{}
	seenHost := map[string]bool{}

	addHost := func(h string) {
		host := hostOf(h)
		if host == "" || seenHost[host] {
			return
		}
		seenHost[host] = true
		ctx.Hosts = append(ctx.Hosts, host)
	}

	var fileVars map[string]string
	for _, r := range httpfile.ParseFile(src) {
		for _, h := range r.Headers {
			ctx.Headers = AppendUnique(ctx.Headers, h.Name)
		}
		addHost(r.URL)
		if r.Vars != nil {
			fileVars = r.Vars
		}
	}
	for name, val := range fileVars {
		ctx.Vars = AppendUnique(ctx.Vars, name)
		addHost(val)
	}
	for name, val := range profileVars {
		ctx.Vars = AppendUnique(ctx.Vars, name)
		addHost(val)
	}

	ctx.Methods = Methods()
	for _, fn := range BuiltinFuncs() {
		ctx.Vars = AppendUnique(ctx.Vars, fn)
	}

	// The maps above iterate in random order; sort so the popup list is stable
	// between openings.
	sort.Strings(ctx.Hosts)
	sort.Strings(ctx.Vars)
	sort.Strings(ctx.Headers)
	return ctx
}

// Methods returns the HTTP method names offered for completion.
func Methods() []string {
	return []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"}
}

// builtinFuncs are the {{$fn}} built-in variable functions offered for variable
// completion.
var builtinFuncs = []string{
	"$timestamp",
	"$isoTimestamp",
	"$random.uuid",
	"$random.int",
	"$guid",
	"b64",
}

// BuiltinFuncs returns the built-in {{$fn}} variable functions.
func BuiltinFuncs() []string { return append([]string(nil), builtinFuncs...) }

// hostOf extracts a bare hostname from a URL string or a raw host value.
func hostOf(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// AppendUnique appends s to list when it is not already present.
func AppendUnique(list []string, s string) []string {
	for _, e := range list {
		if e == s {
			return list
		}
	}
	return append(list, s)
}
