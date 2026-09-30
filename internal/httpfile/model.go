package httpfile

// Header is a single HTTP request header (name/value pair).
type Header struct {
	Name  string
	Value string
}

// Request is one parsed HTTP request block from a .http file.
type Request struct {
	// Name is the optional @name annotation for this request block.
	Name string

	// Method is the HTTP method (GET, POST, ...).
	Method string

	// URL is the request target (may contain {{placeholders}}).
	URL string

	// Headers are the request headers in source order.
	Headers []Header

	// Body is the raw request body ("" if none).
	Body string

	// BodyStart / BodyEnd are the 1-based source lines of the body, inclusive.
	// Both are 0 when there is no body block. Used to locate/replace the body
	// text in the editor (e.g. for JSON formatting).
	BodyStart int
	BodyEnd   int

	// Line is the 1-based source line where the request line starts.
	Line int
}

// IsEmpty reports whether this request block has no request line yet.
func (r *Request) IsEmpty() bool {
	return r.Method == "" && r.URL == ""
}