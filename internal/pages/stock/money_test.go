package stock

import (
	"math"
	"testing"
)

// TestMoneyPutsTheSignInFrontOfTheSymbol pins the application's one money
// format: a negative reading is -¥1.00, never ¥-1.00, so the sign sits in the
// same place on the account page, the statistics cards and the chart tip.
func TestMoneyPutsTheSignInFrontOfTheSymbol(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "¥0.00"},
		{12345, "¥123.45"},
		{-12345, "-¥123.45"},
		{math.MinInt64, "-¥92233720368547758.08"},
	}
	for _, one := range cases {
		if got := money(one.cents); got != one.want {
			t.Errorf("money(%d) = %q, want %q", one.cents, got, one.want)
		}
	}
}

// TestSignedMoneyNamesTheDirection covers the delta form: a change always says
// which way it went, and still puts the sign in front of the symbol.
func TestSignedMoneyNamesTheDirection(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "+¥0.00"},
		{500, "+¥5.00"},
		{-500, "-¥5.00"},
		{math.MinInt64, "-¥92233720368547758.08"},
	}
	for _, one := range cases {
		if got := signedMoney(one.cents); got != one.want {
			t.Errorf("signedMoney(%d) = %q, want %q", one.cents, got, one.want)
		}
	}
}
