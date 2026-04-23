// simpanan-webui is the long-lived browser-based client for simpanan.
// It runs as a localhost server on a fixed port and lets the user
// drive the same backend the Neovim rplugin uses, from a browser tab.
//
// See specs/webui.allium for the behavioural specification.
package main

import (
	"fmt"
	"os"

	"simpanan/internal/webui"
)

func main() {
	srv := webui.NewServer(webui.DefaultPort)
	if err := srv.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
