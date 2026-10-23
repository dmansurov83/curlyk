// Package help holds the key-reference contents shared by the console and the
// desktop UI: which key combinations are listed, and which i18n keys describe
// them. It is deliberately free of any UI dependency, so the TUI can render it
// with lipgloss and the Fyne window with its own widgets.
package help

// Entry is one row of the key reference: the literal key combination (never
// localised) and the i18n key of the action it performs.
type Entry struct {
	Key     string
	DescKey string
}

// VarLine is one row of the variables section. A literal row carries the syntax
// itself (e.g. "@var name = value") and is rendered in the key style; the rest
// are i18n descriptions rendered in the muted style.
type VarLine struct {
	Key     string
	Literal bool
}

// TUIKeys is the ordered key reference of the console UI. It mirrors the README
// table and the bindings in internal/tui/keys.go.
var TUIKeys = []Entry{
	{Key: "Ctrl+Enter / Ctrl+R", DescKey: "help.run"},
	{Key: "Enter", DescKey: "help.runMenu"},
	{Key: "Ctrl+K", DescKey: "help.copyCurl"},
	{Key: "Ctrl+G", DescKey: "help.nav"},
	{Key: "Ctrl+Space", DescKey: "help.complete"},
	{Key: "Type to search", DescKey: "help.find"},
	{Key: "Ctrl+Z", DescKey: "help.undo"},
	{Key: "Ctrl+Shift+Z", DescKey: "help.redo"},
	{Key: "Ctrl+Y", DescKey: "help.delLine"},
	{Key: "Ctrl+C / Ctrl+X / Ctrl+V", DescKey: "help.copy"},
	{Key: "Ctrl+S", DescKey: "help.save"},
	{Key: "Ctrl+N", DescKey: "help.new"},
	{Key: "Tab", DescKey: "help.pane"},
	{Key: "Ctrl+L", DescKey: "help.lang"},
	{Key: "Ctrl+] / Ctrl+\\", DescKey: "help.theme"},
	{Key: "Ctrl+D", DescKey: "help.debug"},
	{Key: "Ctrl+←/→", DescKey: "help.words"},
	{Key: "Home", DescKey: "help.home"},
	{Key: "Alt+←/→", DescKey: "help.hscroll"},
	{Key: "F1", DescKey: "help.panel"},
	{Key: "Esc Esc / F10", DescKey: "help.quit"},
}

// GUIKeys is the ordered key reference of the desktop window. The window follows
// the platform shortcuts there they differ from the console (Ctrl+T cycles the
// scheme, Ctrl+Y redoes) and has no delete-line or debug-dump bindings.
var GUIKeys = []Entry{
	{Key: "Ctrl+Enter / Ctrl+R", DescKey: "help.run"},
	{Key: "Right click / ▶", DescKey: "help.runMenu"},
	{Key: "Ctrl+K", DescKey: "help.copyCurl"},
	{Key: "Ctrl+G", DescKey: "help.nav"},
	{Key: "Ctrl+Space", DescKey: "help.complete"},
	{Key: "Ctrl+F", DescKey: "help.find"},
	{Key: "Ctrl+Z", DescKey: "help.undo"},
	{Key: "Ctrl+Y / Ctrl+Shift+Z", DescKey: "help.redo"},
	{Key: "Ctrl+C / Ctrl+X / Ctrl+V", DescKey: "help.copy"},
	{Key: "Ctrl+S", DescKey: "help.save"},
	{Key: "Ctrl+O / Ctrl+N", DescKey: "help.new"},
	{Key: "Tab", DescKey: "help.pane"},
	{Key: "Ctrl+L", DescKey: "help.lang"},
	{Key: "Ctrl+T", DescKey: "help.theme"},
	{Key: "Ctrl+←/→", DescKey: "help.words"},
	{Key: "Home", DescKey: "help.home"},
	{Key: "F1", DescKey: "help.panel"},
}

// VarLines is the variables section of the reference, in display order.
var VarLines = []VarLine{
	{Key: "help.varDecl", Literal: true},
	{Key: "help.varUUID"},
	{Key: "help.varInt"},
	{Key: "help.varTime"},
	{Key: "help.varB64"},
}
