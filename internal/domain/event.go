package domain

import "strings"

// KeyEventTitleLimit is how many Unicode code points a key event's title keeps.
// The application truncated with Array.from(...).slice(0, 200), which is
// code points, not bytes.
const KeyEventTitleLimit = 200

// KeyEventTitle is the name an event is stored and listed under: the title the
// user typed, or — when that is blank — the first non-empty line of the body.
// The result keeps at most KeyEventTitleLimit code points. A list card, an
// event that only has a body, and an event that has neither all agree on what
// the date is called.
func KeyEventTitle(title, content string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		for _, line := range strings.Split(content, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				name = line
				break
			}
		}
	}
	return TruncateRunes(name, KeyEventTitleLimit)
}

// TruncateRunes keeps at most limit code points of value. It is the rune-based
// form of the application's slice(0, n) on the strings users type.
func TruncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
