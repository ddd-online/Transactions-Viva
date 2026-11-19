// Package icons holds the application's compiled line icons. The SVG sources
// under svg/ are the editable form; regenerate icons.go after changing them.
package icons

//go:generate go tool viva icons -o icons.go -pkg icons -license NOTICE svg
