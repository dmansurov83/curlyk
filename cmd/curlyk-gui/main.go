// Command curlyk-gui is the desktop UI of curlyk: a Fyne window around the same
// .http sources the TUI edits, sharing appsettings.yml with it.
package main

import (
	"log"

	"github.com/user/curlyk/internal/gui"
)

func main() {
	if err := gui.Run(); err != nil {
		log.Fatal(err)
	}
}
