// Command transactions is the application entry point.
package main

import (
	"log"

	"transactions/internal/desktop"
)

func main() {
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}
