package domain

import (
	"errors"
	"testing"
)

// 这些用例直接搬自本项目 tests/unit/money.spec.ts：行为是硬契约，
// 不应该「顺手」改变取整或格式化的结果。

func TestCentsToYuan(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{12345, "123.45"},
		{-5, "-0.05"},
		{-12345, "-123.45"},
		{100_000_000, "1000000.00"},
	}
	for _, test := range cases {
		if got := CentsToYuan(test.cents); got != test.want {
			t.Errorf("CentsToYuan(%d) = %q, want %q", test.cents, got, test.want)
		}
	}
}

func TestYuanToCents(t *testing.T) {
	cases := []struct {
		input string
		want  int64
	}{
		{"123.45", 12345},
		{" 123 ", 12300},
		{"-0.05", -5},
		{".5", 50},
		{"0.5", 50},
		{"1.", 100},
		{"1", 100},
		{"", 0},
		{"   ", 0},
		{"0.004", 0},
		{"0.005", 1},
		{"0.014", 1},
		{"0.015", 2},
		{"0.999", 100},
		{"0.0049", 0},
		{"0.0051", 1},
	}
	for _, test := range cases {
		got, err := YuanToCents(test.input)
		if err != nil {
			t.Fatalf("YuanToCents(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Errorf("YuanToCents(%q) = %d, want %d", test.input, got, test.want)
		}
	}
}

func TestYuanToCentsRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"abc", "1.2.3", "1a", "--1"} {
		if _, err := YuanToCents(input); !errors.Is(err, ErrInvalidMoney) {
			t.Errorf("YuanToCents(%q) error = %v, want ErrInvalidMoney", input, err)
		}
	}
}

func TestYuanToCentsRejectsOverflow(t *testing.T) {
	if _, err := YuanToCents("99999999999999999999"); !errors.Is(err, ErrMoneyOverflow) {
		t.Fatalf("overflow error = %v, want ErrMoneyOverflow", err)
	}
}

func TestPricesInMilli(t *testing.T) {
	prices := []struct {
		yuan float64
		want int64
	}{
		{10.0, 10_000},
		{38.06, 38_060},
		{0.0, 0},
		{4.389, 4_389},
		{1.537, 1_537},
		{0.528, 528},
		{4.3894, 4_389},
		{4.3896, 4_390},
		{-12.345, -12_345},
	}
	for _, test := range prices {
		if got := PriceYuanToMilli(test.yuan); got != test.want {
			t.Errorf("PriceYuanToMilli(%v) = %d, want %d", test.yuan, got, test.want)
		}
	}

	formatted := []struct {
		milli int64
		want  string
	}{
		{0, "0.00"},
		{10_000, "10.00"},
		{10_050, "10.05"},
		{100_000, "100.00"},
		{4_389, "4.389"},
		{1_537, "1.537"},
		{528, "0.528"},
		{-4_389, "-4.389"},
		{-10_000, "-10.00"},
	}
	for _, test := range formatted {
		if got := MilliToYuan(test.milli); got != test.want {
			t.Errorf("MilliToYuan(%d) = %q, want %q", test.milli, got, test.want)
		}
	}

	for _, milli := range []int64{-12_345, -1, 0, 528, 4_389, 100_000} {
		if got := PriceYuanToMilli(MilliToPriceYuan(milli)); got != milli {
			t.Errorf("roundtrip %d = %d", milli, got)
		}
	}
}

func TestMoneyRoundtrip(t *testing.T) {
	for _, cents := range []int64{-99_999, -1, 0, 1, 7, 100, 12_345, 987_654} {
		got, err := YuanToCents(CentsToYuan(cents))
		if err != nil {
			t.Fatal(err)
		}
		if got != cents {
			t.Errorf("roundtrip %d = %d", cents, got)
		}
	}
}
