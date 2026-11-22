//go:build !windows

package proxy

// systemProxy is empty on platforms where the environment is the only
// documented automatic source.
func systemProxy() string { return "" }
