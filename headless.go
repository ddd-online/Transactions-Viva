//go:build vivahead

package main

// Importing github.com/vivaui/viva/headless/auto makes the application serve
// the headless line protocol instead of opening a window when the process
// starts with VIVA_HEADLESS set. It is behind a build tag so release builds
// never contain it:
//
//	go build -tags vivahead -o /tmp/transactions-headless .
//	VIVA_HEADLESS=1 /tmp/transactions-headless
//
// Then send commands such as `tree`, `layout`, `texts`, `shot PATH` and `quit`.
import (
	_ "github.com/vivaui/viva/headless/auto"

	"transactions/internal/desktop"
)

// init marks this build as a test instrument. It reads the development
// configuration and refuses to open the production workspace, so a headless run
// can never migrate a person's real workspace or write test data into it.
func init() { desktop.UseHeadlessConfig() }
