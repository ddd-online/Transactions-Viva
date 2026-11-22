package domain

import "testing"

func TestPlanLevelsOf(t *testing.T) {
	cases := []struct {
		name, buy, stop, take string
		upside, downside      string
		kind                  PlanRatioKind
		ratio                 string
	}{
		{
			name: "readable", buy: "12.98", stop: "12.35", take: "14",
			upside: "+7.86%", downside: "-4.85%", kind: PlanRatioValue, ratio: "1.62",
		},
		{
			name: "three decimals and plus", buy: "+12.98", stop: "12.355", take: "14.000",
			upside: "+7.86%", downside: "-4.82%", kind: PlanRatioValue, ratio: "1.63",
		},
		{
			name: "level on cost", buy: "10", stop: "10", take: "10",
			upside: "0.00%", downside: "0.00%", kind: PlanRatioBlocked,
		},
		{
			name: "missing levels", buy: "10", stop: "", take: "",
			kind: PlanRatioMissing,
		},
		{
			name: "one level missing", buy: "10", stop: "9", take: "",
			downside: "-10.00%", kind: PlanRatioMissing,
		},
		{
			name: "not prices", buy: "10", stop: "破 9 就走", take: "12",
			upside: "+20.00%", kind: PlanRatioUnreadable,
		},
		{
			name: "wrong direction", buy: "10", stop: "9.5", take: "10.5",
			upside: "+5.00%", downside: "-5.00%", kind: PlanRatioValue, ratio: "1.00",
		},
		{
			name: "target below cost", buy: "10", stop: "9", take: "9.5",
			upside: "-5.00%", downside: "-10.00%", kind: PlanRatioBlocked,
		},
		{
			name: "stop above cost", buy: "10", stop: "10.5", take: "12",
			upside: "+20.00%", downside: "+5.00%", kind: PlanRatioBlocked,
		},
		{
			name: "no buy price", buy: "", stop: "9", take: "12",
			kind: PlanRatioMissing,
		},
		{
			name: "invalid buy price", buy: "abc", stop: "9", take: "12",
			kind: PlanRatioMissing,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			levels := PlanLevelsOf(test.buy, test.stop, test.take)
			if levels.Upside != test.upside || levels.Downside != test.downside {
				t.Fatalf("upside/downside = %q/%q, want %q/%q",
					levels.Upside, levels.Downside, test.upside, test.downside)
			}
			if levels.RatioKind != test.kind || levels.RatioText != test.ratio {
				t.Fatalf("ratio = %v %q, want %v %q",
					levels.RatioKind, levels.RatioText, test.kind, test.ratio)
			}
		})
	}
}
