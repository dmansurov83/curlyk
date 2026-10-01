package curl

import "strings"

// Parsed is the result of analysing a cURL command line.
type Parsed struct {
	URL    string
	Method string
	// Headers in source order (only explicit -H).
	Headers []Header
	// Data is the body from -d / --data* flags.
	Data string
	// FormFields are multipart parts from -F/--form.
	FormFields []FormField
	User       string
	// Form is true when any -F was present.
	Form bool
}

// Header mirrors one -H value.
type Header struct{ Name, Value string }

// FormField is one -F part (field name, value and optional filename/content-type).
type FormField struct {
	Name     string
	Value    string
	Filename string
	HasFile  bool
}

// ImportCommand parses the given argv (without "curl" itself).
func ImportCommand(argv []string) (*Parsed, error) {
	p := &Parsed{Method: "GET"}
	i := 0

	hasMore := func() bool { return i < len(argv) }
	next := func() string { v := argv[i]; i++; return v }

	// optionValue returns the value for an option that takes one:
	// either from --opt=value (inline) or the next argv. It advances i only
	// when consuming the next argv.
	optionVal := func(arg string) (string, bool) {
		if strings.HasPrefix(arg, "--") {
			if eq := strings.IndexRune(arg, '='); eq >= 0 {
				return arg[eq+1:], true
			}
		}
		if hasMore() {
			return next(), true
		}
		return "", false
	}

	setMethod := func(m string) { p.Method = strings.ToUpper(m) }
	dataPOST := func() {
		if p.Method == "GET" {
			p.Method = "POST"
		}
	}

	for hasMore() {
		arg := next()

		switch {
		case arg == "--":
			if hasMore() {
				if p.URL == "" {
					p.URL = next()
				} else {
					next()
				}
			}
			continue

		case arg == "-X" || arg == "--request":
			if v, ok := optionVal(arg); ok {
				setMethod(v)
			}
		case arg == "-H" || arg == "--header":
			if v, ok := optionVal(arg); ok {
				addHeader(p, v)
			}
		case arg == "-d" || arg == "--data" || arg == "--data-raw":
			if v, ok := optionVal(arg); ok {
				p.Data += v
				dataPOST()
			}
		case arg == "--data-binary" || arg == "--data-urlencode":
			if v, ok := optionVal(arg); ok {
				p.Data += v
				dataPOST()
			}
		case arg == "-u" || arg == "--user":
			if v, ok := optionVal(arg); ok {
				p.User = v
			}
		case arg == "-F" || arg == "--form" || arg == "--form-string":
			if v, ok := optionVal(arg); ok {
				addFormField(p, v)
			}
		case arg == "-I" || arg == "--head":
			setMethod("HEAD")
		case arg == "-G" || arg == "--get":
			p.Method = "GET"
		case arg == "-o" || arg == "--output" || arg == "-b" || arg == "--cookie" ||
			arg == "-c" || arg == "--cookie-jar" || arg == "-A" || arg == "--user-agent" ||
			arg == "-e" || arg == "--referer" || arg == "--max-time" || arg == "-m":
			optionVal(arg)
		case arg == "--compressed" || arg == "-L" || arg == "--location" ||
			arg == "-s" || arg == "--silent" || arg == "-i" || arg == "--include" ||
			arg == "-k" || arg == "--insecure" || arg == "-v" || arg == "--verbose" ||
			arg == "--show-error":
			// ignored flags without values

		case strings.HasPrefix(arg, "-X") && len(arg) > 2:
			setMethod(arg[2:])
		case strings.HasPrefix(arg, "-H") && len(arg) > 2:
			addHeader(p, arg[2:])
		case strings.HasPrefix(arg, "-d") && len(arg) > 2:
			p.Data += arg[2:]
			dataPOST()
		case strings.HasPrefix(arg, "-u") && len(arg) > 2:
			p.User = arg[2:]
		case strings.HasPrefix(arg, "-F") && len(arg) > 2:
			addFormField(p, arg[2:])

		case !strings.HasPrefix(arg, "-"):
			if p.URL == "" {
				p.URL = arg
			}
		}
	}
	return p, nil
}

// ImportLine parses a full cURL command line (starting with "curl") and
// returns the equivalent .http block source.
func ImportLine(line string) (string, error) {
	argv, err := ShellSplit(line)
	if err != nil {
		return "", err
	}
	if len(argv) > 0 {
		first := strings.ToLower(argv[0])
		if first == "curl" || first == "curl.exe" {
			argv = argv[1:]
		}
	}
	p, err := ImportCommand(argv)
	if err != nil {
		return "", err
	}
	return ToHTTPString(p), nil
}

func addHeader(p *Parsed, h string) {
	h = strings.TrimSpace(h)
	if idx := strings.IndexRune(h, ':'); idx >= 0 {
		p.Headers = append(p.Headers, Header{Name: strings.TrimSpace(h[:idx]), Value: strings.TrimSpace(h[idx+1:])})
	} else {
		p.Headers = append(p.Headers, Header{Name: h, Value: ""})
	}
}

// addFormField parses a -F part spec: name=value, name=@file, name=@file;type=...
func addFormField(p *Parsed, s string) {
	p.Form = true
	name, rest, _ := strings.Cut(s, "=")
	name = strings.TrimSpace(name)
	rest = strings.TrimSpace(rest)

	f := FormField{Name: name}
	if strings.HasPrefix(rest, "@") {
		fileSpec := rest[1:]
		// split off ;type= / ;filename=
		if sc := strings.IndexRune(fileSpec, ';'); sc >= 0 {
			f.Filename = fileSpec[:sc]
		} else {
			f.Filename = fileSpec
		}
		f.HasFile = true
		p.FormFields = append(p.FormFields, f)
		return
	}
	f.Value = rest
	p.FormFields = append(p.FormFields, f)
}
