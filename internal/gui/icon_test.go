package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// TestRequestRowsMarkRequestStarts covers the icon source: only the lines that
// open a request get the ▶ marker.
func TestRequestRowsMarkRequestStarts(t *testing.T) {
	e := newTestEditor(t, "### One\nGET http://one\nAccept: */*\n\n### Two\nPOST http://two\n")
	for row := 0; row < e.LineCount(); row++ {
		want := row == 1 || row == 5
		if got := e.requestRows[row]; got != want {
			t.Errorf("row %d marked = %v, want %v", row, got, want)
		}
	}
}

// clickIcon sends the MouseDown+Tapped pair Fyne produces for a primary click.
func clickIcon(e *Editor, pos fyne.Position) {
	e.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: pos},
		Button:     desktop.MouseButtonPrimary,
	})
	e.Tapped(&fyne.PointEvent{Position: pos})
}

// TestClickOnRunIconOpensTheMenu covers the icon hit-test before the caret math.
func TestClickOnRunIconOpensTheMenu(t *testing.T) {
	u := newTestUI(t)
	u.ed.Resize(fyne.NewSize(600, 300))
	u.ed.SetText("GET http://one\n")

	clickIcon(u.ed, fyne.NewPos(paddingX, paddingY))

	if !u.requestMenuOpen() {
		t.Fatal("a click on the ▶ icon did not open the request menu")
	}
	u.closeRequestMenu()
}

// TestClickBesideTheIconMovesTheCaret keeps the gutter hit area narrow: a click
// on the text moves the caret instead of opening the menu.
func TestClickBesideTheIconMovesTheCaret(t *testing.T) {
	u := newTestUI(t)
	u.ed.Resize(fyne.NewSize(600, 300))
	u.ed.SetText("GET http://one\n")
	u.ed.doc.setCaret(0, 0)

	clickIcon(u.ed, fyne.NewPos(u.ed.textOriginX()+20, paddingY))

	if u.requestMenuOpen() {
		t.Fatal("a click beside the icon opened the menu")
	}
	if row, col := u.ed.Caret(); row != 0 || col == 0 {
		t.Fatalf("caret = %d:%d, want it moved into the text", row, col)
	}
}
