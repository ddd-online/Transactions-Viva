package ids

import (
	"regexp"
	"testing"
)

func TestNewMakesVersionFourUUIDs(t *testing.T) {
	shape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 100 {
		id := New()
		if !shape.MatchString(id) {
			t.Fatalf("id %q is not a version 4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("id %q came back twice", id)
		}
		seen[id] = true
	}
}
