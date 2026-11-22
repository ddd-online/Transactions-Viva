// Command transactions is the application entry point.
package main

import (
	"flag"
	"log"

	"transactions/internal/desktop"
)

func main() {
	// `viva dev .` sets VIVA_RELOAD_ADDRESS and is handled inside the shell;
	// -dev is for `go run .` / IDE runs, which open a native window but should
	// still read ~/.transactions.dev.json like development builds do.
	dev := flag.Bool("dev", false,
		"use the development configuration (~/.transactions.dev.json)")
	flag.Parse()
	if err := desktop.Run(*dev); err != nil {
		log.Fatal(err)
	}
}
