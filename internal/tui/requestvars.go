package tui

import (
	"errors"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// resolveRequestVars substitutes every {{name}} / {{$fn}} placeholder in a
// request with concrete values, in place: the URL, each header value and the
// body. Values come from the file's @var declarations merged with the active
// profile's (@var in the profile fills in whatever the file does not define; the
// file wins — see httpfile.MergeVars).
//
// It is the single implementation shared by "run" and "copy as cURL", so both
// build the same request from the same source. On the first unresolved
// placeholder it returns an error and leaves the request alone, so the caller
// aborts instead of putting a raw "{{x}}" on the wire.
func (m *model) resolveRequestVars(req *httpfile.Request) error {
	vars := req.Vars
	if pv := m.activeProfileVars(); len(pv) > 0 {
		vars = httpfile.MergeVars(pv, vars)
	}
	sub, err := httpfile.Substitute(req.URL, vars)
	if err != nil {
		return err
	}
	req.URL = sub
	for i := range req.Headers {
		hv, herr := httpfile.Substitute(req.Headers[i].Value, vars)
		if herr != nil {
			return herr
		}
		req.Headers[i].Value = hv
	}
	if req.Body != "" {
		bv, berr := httpfile.Substitute(req.Body, vars)
		if berr != nil {
			return berr
		}
		req.Body = bv
	}
	return nil
}

// substitutionMessage renders the status-bar text for a substitution failure:
// the name of the first unresolved variable when the error carries one, and the
// error text otherwise. The lookup goes through errors.As so an error of another
// type can never panic the UI.
func substitutionMessage(err error) string {
	var se *httpfile.SubstitutionError
	if errors.As(err, &se) {
		return i18n.T("err.variable", se.First())
	}
	return i18n.T("err.variable", err.Error())
}
