// Package theme defines application color schemes for the curlyk TUI. A
// scheme is a plain-data set of ANSI-256 color numbers (as strings, "0"–"255")
// matching every hardcoded lipgloss color in the UI. Styles are rebuilt from
// the currently selected scheme at runtime, so switching is immediate.
//
// Three builtin schemes ship with the app: "default" (the original palette,
// identical behaviour to the code before theming), "darkula" (a Darcula-like
// dark theme) and "light". Users can add their own schemes as YAML files in
// `themes/<name>.theme`; a partial file only overrides the fields it lists and
// inherits the rest from "default".
package theme

// Names of the builtin schemes.
const (
	DefaultName = "default"
	DarkulaName = "darkula"
	LightName   = "light"
	BuiltinDir  = "themes"
	ThemeExt    = ".theme"
)

// Scheme is a complete set of ANSI-256 color numbers for the UI. Field names
// are the YAML keys used in `themes/*.theme` files.
type Scheme struct {
	Method      string `yaml:"method"`
	URL         string `yaml:"url"`
	HTTPVer     string `yaml:"http_ver"`
	HeaderName  string `yaml:"header_name"`
	HeaderValue string `yaml:"header_value"`
	Body        string `yaml:"body"`
	Comment     string `yaml:"comment"`
	Variable    string `yaml:"variable"`
	Separator   string `yaml:"separator"`
	Option      string `yaml:"option"`
	CursorBg    string `yaml:"cursor_bg"`
	CursorFg    string `yaml:"cursor_fg"`
	SelectionBg string `yaml:"selection_bg"`

	JSONKey    string `yaml:"json_key"`
	JSONString string `yaml:"json_string"`
	JSONNumber string `yaml:"json_number"`
	JSONBool   string `yaml:"json_bool"`
	JSONNull   string `yaml:"json_null"`
	JSONPunct  string `yaml:"json_punct"`
	JSONSelBg  string `yaml:"json_selection_bg"`

	RunIcon      string `yaml:"run_icon"`
	IconBlockBg  string `yaml:"icon_block_bg"`
	IconBlockFg  string `yaml:"icon_block_fg"`
	NumCurrent   string `yaml:"num_current"`
	NumMuted     string `yaml:"num_muted"`
	ScrollThumb  string `yaml:"scroll_thumb"`
	ScrollTrack  string `yaml:"scroll_track"`
	HeaderBg     string `yaml:"header_bg"`
	HeaderFg     string `yaml:"header_fg"`
	StatusBg     string `yaml:"status_bg"`
	StatusFg     string `yaml:"status_fg"`
	BorderActive string `yaml:"border_active"`
	BorderIdle   string `yaml:"border_idle"`

	FileSelected string `yaml:"file_selected"`
	FileHoverFg  string `yaml:"file_hover_fg"`
	FileHoverBg  string `yaml:"file_hover_bg"`

	Executing   string `yaml:"executing"`
	CopyBtnFg   string `yaml:"copy_btn_fg"`
	CopyBtnBg   string `yaml:"copy_btn_bg"`
	CopyHoverFg string `yaml:"copy_hover_fg"`
	CopyHoverBg string `yaml:"copy_hover_bg"`

	MenuSelFg   string `yaml:"menu_sel_fg"`
	MenuSelBg   string `yaml:"menu_sel_bg"`
	MenuHoverFg string `yaml:"menu_hover_fg"`
	MenuHoverBg string `yaml:"menu_hover_bg"`
	MenuFg      string `yaml:"menu_fg"`
	MenuBg      string `yaml:"menu_bg"`

	FindFg      string `yaml:"find_fg"`
	FindBg      string `yaml:"find_bg"`
	FindCurFg   string `yaml:"find_cur_fg"`
	FindCurBg   string `yaml:"find_cur_bg"`
	SearchBarBg string `yaml:"search_bar_bg"`
	SearchBarFg string `yaml:"search_bar_fg"`

	HelpKey   string `yaml:"help_key"`
	HelpDesc  string `yaml:"help_desc"`
	HelpTitle string `yaml:"help_title"`

	ProfileSep    string `yaml:"profile_sep"`
	ProfileTitle  string `yaml:"profile_title"`
	ProfileActive string `yaml:"profile_active"`
	ProfileSel    string `yaml:"profile_sel"`
	ProfileNew    string `yaml:"profile_new"`

	DialogTitle  string `yaml:"dialog_title"`
	DialogBorder string `yaml:"dialog_border"`
}

// Default returns the original UI palette (identical to pre-theming behaviour).
func Default() Scheme {
	return Scheme{
		Method:      "212",
		URL:         "39",
		HTTPVer:     "245",
		HeaderName:  "80",
		HeaderValue: "188",
		Body:        "186",
		Comment:     "240",
		Variable:    "214",
		Separator:   "99",
		Option:      "178",
		CursorBg:    "63",
		CursorFg:    "15",
		SelectionBg: "24",

		JSONKey:    "81",
		JSONString: "114",
		JSONNumber: "214",
		JSONBool:   "211",
		JSONNull:   "211",
		JSONPunct:  "240",
		JSONSelBg:  "24",

		RunIcon:      "212",
		IconBlockBg:  "63",
		IconBlockFg:  "15",
		NumCurrent:   "212",
		NumMuted:     "240",
		ScrollThumb:  "250",
		ScrollTrack:  "240",
		HeaderBg:     "235",
		HeaderFg:     "252",
		StatusBg:     "236",
		StatusFg:     "252",
		BorderActive: "212",
		BorderIdle:   "240",

		FileSelected: "212",
		FileHoverFg:  "212",
		FileHoverBg:  "238",

		Executing:   "214",
		CopyBtnFg:   "255",
		CopyBtnBg:   "240",
		CopyHoverFg: "15",
		CopyHoverBg: "30",

		MenuSelFg:   "15",
		MenuSelBg:   "63",
		MenuHoverFg: "15",
		MenuHoverBg: "60",
		MenuFg:      "252",
		MenuBg:      "237",

		FindFg:      "232",
		FindBg:      "220",
		FindCurFg:   "255",
		FindCurBg:   "196",
		SearchBarBg: "235",
		SearchBarFg: "222",

		HelpKey:   "212",
		HelpDesc:  "245",
		HelpTitle: "255",

		ProfileSep:    "240",
		ProfileTitle:  "245",
		ProfileActive: "212",
		ProfileSel:    "245",
		ProfileNew:    "212",

		DialogTitle:  "255",
		DialogBorder: "212",
	}
}

// Darkula returns a dark, Darcula-inspired palette (JetBrains style).
func Darkula() Scheme {
	return Scheme{
		Method:      "167",
		URL:         "75",
		HTTPVer:     "110",
		HeaderName:  "115",
		HeaderValue: "152",
		Body:        "187",
		Comment:     "102",
		Variable:    "221",
		Separator:   "141",
		Option:      "221",
		CursorBg:    "141",
		CursorFg:    "16",
		SelectionBg: "60",

		JSONKey:    "197",
		JSONString: "187",
		JSONNumber: "179",
		JSONBool:   "205",
		JSONNull:   "205",
		JSONPunct:  "250",
		JSONSelBg:  "60",

		RunIcon:      "167",
		IconBlockBg:  "141",
		IconBlockFg:  "16",
		NumCurrent:   "141",
		NumMuted:     "102",
		ScrollThumb:  "145",
		ScrollTrack:  "59",
		HeaderBg:     "236",
		HeaderFg:     "252",
		StatusBg:     "237",
		StatusFg:     "252",
		BorderActive: "141",
		BorderIdle:   "59",

		FileSelected: "167",
		FileHoverFg:  "167",
		FileHoverBg:  "238",

		Executing:   "179",
		CopyBtnFg:   "255",
		CopyBtnBg:   "59",
		CopyHoverFg: "16",
		CopyHoverBg: "142",

		MenuSelFg:   "16",
		MenuSelBg:   "141",
		MenuHoverFg: "16",
		MenuHoverBg: "60",
		MenuFg:      "252",
		MenuBg:      "237",

		FindFg:      "232",
		FindBg:      "221",
		FindCurFg:   "16",
		FindCurBg:   "197",
		SearchBarBg: "236",
		SearchBarFg: "221",

		HelpKey:   "141",
		HelpDesc:  "250",
		HelpTitle: "255",

		ProfileSep:    "59",
		ProfileTitle:  "250",
		ProfileActive: "167",
		ProfileSel:    "250",
		ProfileNew:    "167",

		DialogTitle:  "255",
		DialogBorder: "141",
	}
}

// Light returns a light background palette.
func Light() Scheme {
	return Scheme{
		Method:      "124",
		URL:         "25",
		HTTPVer:     "59",
		HeaderName:  "30",
		HeaderValue: "23",
		Body:        "22",
		Comment:     "102",
		Variable:    "88",
		Separator:   "99",
		Option:      "94",
		CursorBg:    "33",
		CursorFg:    "255",
		SelectionBg: "188",

		JSONKey:    "19",
		JSONString: "22",
		JSONNumber: "88",
		JSONBool:   "90",
		JSONNull:   "90",
		JSONPunct:  "240",
		JSONSelBg:  "188",

		RunIcon:      "124",
		IconBlockBg:  "33",
		IconBlockFg:  "255",
		NumCurrent:   "124",
		NumMuted:     "102",
		ScrollThumb:  "59",
		ScrollTrack:  "250",
		HeaderBg:     "252",
		HeaderFg:     "16",
		StatusBg:     "254",
		StatusFg:     "16",
		BorderActive: "124",
		BorderIdle:   "246",

		FileSelected: "124",
		FileHoverFg:  "124",
		FileHoverBg:  "254",

		Executing:   "88",
		CopyBtnFg:   "255",
		CopyBtnBg:   "59",
		CopyHoverFg: "255",
		CopyHoverBg: "33",

		MenuSelFg:   "255",
		MenuSelBg:   "33",
		MenuHoverFg: "255",
		MenuHoverBg: "25",
		MenuFg:      "16",
		MenuBg:      "254",

		FindFg:      "232",
		FindBg:      "220",
		FindCurFg:   "255",
		FindCurBg:   "196",
		SearchBarBg: "252",
		SearchBarFg: "16",

		HelpKey:   "124",
		HelpDesc:  "102",
		HelpTitle: "16",

		ProfileSep:    "246",
		ProfileTitle:  "59",
		ProfileActive: "124",
		ProfileSel:    "59",
		ProfileNew:    "124",

		DialogTitle:  "16",
		DialogBorder: "124",
	}
}

// ByName returns the builtin scheme for a builtin name. ok is false for any
// name that is not "default"/"darkula"/"light".
func ByName(name string) (Scheme, bool) {
	switch name {
	case DefaultName:
		return Default(), true
	case DarkulaName:
		return Darkula(), true
	case LightName:
		return Light(), true
	default:
		return Scheme{}, false
	}
}

// ByNameOrFile resolves a scheme by name: builtins are returned directly,
// anything else is treated as a custom scheme file `themes/<name>.theme` and
// loaded via LoadFile. Missing/invalid custom files fall back to Default.
func ByNameOrFile(name string) (Scheme, bool) {
	if name == "" {
		return Default(), true
	}
	if s, ok := ByName(name); ok {
		return s, true
	}
	if s, err := LoadFile(BuiltinDir + "/" + name + ThemeExt); err == nil {
		return s, true
	}
	return Default(), false
}
