package typography

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// rawSize finds a text size written as a number where a role belongs: a
// FontSize literal, or a canvas TextStyle's Size literal. The reference
// application's scale exists so that no page has to name a number; a page that
// does is how the 11 / 19 / 22 / 24 / 26 drift accumulated.
var rawSize = regexp.MustCompile(`FontSize\(\s*[0-9]|(^|[^A-Za-z])Size:\s*[0-9]`)

// TestEveryTextUsesARole walks the application's own code and fails on a size
// literal outside this package. Measurements (a control sizing its own grid) may
// still read theme.Metrics; painting text may not.
func TestEveryTextUsesARole(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the package source")
	}
	// .../internal/components/typography/typography_test.go
	internal := filepath.Dir(filepath.Dir(filepath.Dir(source)))
	var offenders []string
	err := filepath.WalkDir(internal, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == filepath.Dir(source) || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(internal, path)
		if err != nil {
			return err
		}
		for index, line := range strings.Split(string(body), "\n") {
			if rawSize.MatchString(line) {
				offenders = append(offenders, relative+":"+strconv.Itoa(index+1)+"  "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("a text size is written as a number; use a role from this package:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// TestTheRampKeepsItsSteps pins the property the roles rely on: two neighbouring
// roles differ by at least 2 px, so no pair reads as the same size.
func TestTheRampKeepsItsSteps(t *testing.T) {
	steps := []struct {
		name string
		size float32
	}{
		{"Caption", SizeCaption},
		{"Body", SizeBody},
		{"Section", SizeSection},
		{"Title", SizeTitle},
		{"Metric", SizeMetric},
	}
	for index := 1; index < len(steps); index++ {
		if gap := steps[index].size - steps[index-1].size; gap < 2 {
			t.Errorf("%s (%g) is %g above %s (%g); the ramp needs at least 2 px",
				steps[index].name, steps[index].size, gap,
				steps[index-1].name, steps[index-1].size)
		}
	}
}
