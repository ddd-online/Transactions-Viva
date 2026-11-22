package events

import "testing"

func TestDetailSplit(t *testing.T) {
	tests := []struct {
		name        string
		hasPictures bool
		hasBody     bool
		pictures    float32
		body        float32
	}{
		{"both take two to one", true, true, 2, 1},
		{"pictures only fill", true, false, 1, 0},
		{"body only fills", false, true, 0, 1},
		{"both keep their empty state", false, false, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pictures, body := detailSplit(test.hasPictures, test.hasBody)
			if pictures != test.pictures || body != test.body {
				t.Fatalf("detailSplit(%v, %v) = %v, %v; want %v, %v",
					test.hasPictures, test.hasBody, pictures, body, test.pictures, test.body)
			}
		})
	}
}
