package tui

import "testing"

func TestMoveWordRight(t *testing.T) {
	ed := newEditor("GET /path here", 40, 10)
	// start at 0 -> word right to end of "GET" (col 3)
	ed.MoveWord("ctrl+right")
	if ed.curCol != 3 {
		t.Errorf("after ctrl+right from 0: col=%d want 3", ed.curCol)
	}
	// again -> to end of "/path" (col 9)
	ed.MoveWord("ctrl+right")
	if ed.curCol != 9 {
		t.Errorf("col=%d want 9", ed.curCol)
	}
	// again -> to end of "here" (col 14)
	ed.MoveWord("ctrl+right")
	if ed.curCol != 14 {
		t.Errorf("col=%d want 14", ed.curCol)
	}
}

func TestMoveWordLeft(t *testing.T) {
	ed := newEditor("GET /path here", 40, 10)
	ed.curCol = 14
	ed.MoveWord("ctrl+left")
	// to start of "here" (col 10)
	if ed.curCol != 10 {
		t.Errorf("col=%d want 10", ed.curCol)
	}
	ed.MoveWord("ctrl+left")
	// to start of "/path" (col 4)
	if ed.curCol != 4 {
		t.Errorf("col=%d want 4", ed.curCol)
	}
	ed.MoveWord("ctrl+left")
	// to start of "GET" (col 0)
	if ed.curCol != 0 {
		t.Errorf("col=%d want 0", ed.curCol)
	}
}

func TestIconNavigation(t *testing.T) {
	ed := newEditor("GET /x", 40, 10)
	ed.curRow, ed.curCol = 0, 2
	// Home twice -> first to col 0, then to icon
	ed.MoveCursor("home")
	if ed.onIcon {
		t.Error("onIcon should be false after first home")
	}
	ed.MoveCursor("home")
	if !ed.onIcon {
		t.Error("onIcon should be true after second home")
	}
	// right from icon -> back to col 0
	ed.MoveCursor("right")
	if ed.onIcon || ed.curCol != 0 {
		t.Errorf("after right from icon: onIcon=%v col=%d", ed.onIcon, ed.curCol)
	}
}

func TestIconNavigationLeft(t *testing.T) {
	ed := newEditor("GET /x", 40, 10)
	ed.curRow, ed.curCol = 0, 0
	// left at col 0, row 0 (top-left) -> to icon
	ed.MoveCursor("left")
	if !ed.onIcon {
		t.Error("left at top-left should go to icon")
	}
	// left again while on icon at top row -> stays
	ed.MoveCursor("left")
	if !ed.onIcon {
		t.Error("should stay on icon")
	}
	// right from icon -> col 0
	ed.MoveCursor("right")
	if ed.onIcon || ed.curCol != 0 {
		t.Errorf("onIcon=%v col=%d", ed.onIcon, ed.curCol)
	}
}