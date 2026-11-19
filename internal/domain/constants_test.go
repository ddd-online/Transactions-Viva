package domain

import "testing"

func TestSnapTodoLevel(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, -1},
		{1, 1},
		{2, 3},
		{4, 5},
		{9, 5},
		{-2, -3},
		{-4, -5},
		{-9, -5},
	}
	for _, test := range cases {
		if got := SnapTodoLevel(test.in); got != test.want {
			t.Errorf("SnapTodoLevel(%d) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTodoLevelLabel(t *testing.T) {
	if got := TodoLevelLabel(-5); got != "低" {
		t.Errorf("TodoLevelLabel(-5) = %q", got)
	}
	if got := TodoLevelLabel(0); got != "次低" {
		t.Errorf("TodoLevelLabel(0) = %q", got)
	}
	if got := TodoLevelLabel(5); got != "高" {
		t.Errorf("TodoLevelLabel(5) = %q", got)
	}
}
