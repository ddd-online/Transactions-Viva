package domain

import "testing"

func TestStockInstrumentAndCode(t *testing.T) {
	cases := []struct {
		code string
		want Instrument
	}{
		{"600000", InstrumentShanghaiStock},
		{"688981", InstrumentShanghaiStock},
		{"000001", InstrumentShenzhenStock},
		{"300750", InstrumentShenzhenStock},
		{"510300", InstrumentShanghaiFund},
		{"159915", InstrumentShenzhenFund},
	}
	for _, test := range cases {
		got, ok := InstrumentOf(test.code)
		if !ok || got != test.want {
			t.Fatalf("InstrumentOf(%q) = %q, %v; want %q", test.code, got, ok, test.want)
		}
		if !IsValidStockCode(test.code) {
			t.Fatalf("IsValidStockCode(%q) = false", test.code)
		}
	}
	for _, code := range []string{"", "60000", "6000000", "830001", "110001", "abc123"} {
		if _, ok := InstrumentOf(code); ok {
			t.Fatalf("InstrumentOf(%q) accepted an unsupported code", code)
		}
	}
	if got := TradeInstrumentOf("830001"); got != InstrumentShenzhenStock {
		t.Fatalf("unknown code fallback = %q", got)
	}
	if got := MarketPrefix("600000"); got != "sh" {
		t.Fatalf("market prefix = %q", got)
	}
	if got := MarketPrefix("000001"); got != "sz" {
		t.Fatalf("market prefix = %q", got)
	}
}

func TestStockAmounts(t *testing.T) {
	cases := []struct {
		price, shares, want int64
	}{
		{3_053, 7, 2_137},
		{5, 1, 1},
		{4, 1, 0},
		{4_389, 100, 43_890},
	}
	for _, test := range cases {
		if got := AmountOf(test.price, test.shares); got != test.want {
			t.Fatalf("AmountOf(%d, %d) = %d, want %d", test.price, test.shares, got, test.want)
		}
	}
	if got := AmountOfLots(4_389, 1); got != 43_890 {
		t.Fatalf("AmountOfLots = %d", got)
	}
	if got := SharesOfLots(3); got != 300 {
		t.Fatalf("SharesOfLots = %d", got)
	}
}

func TestStockOrderFeeAndAllocation(t *testing.T) {
	setting := DefaultStockFeeSetting()
	if got := ComputeCommission(10_000, setting); got != 500 {
		t.Fatalf("minimum commission = %d, want 500", got)
	}
	if got := ComputeCommission(100_000_000, setting); got != 23_540 {
		t.Fatalf("commission = %d, want 23540", got)
	}

	buy := ComputeOrderFee([]int64{100_000_000}, InstrumentShanghaiStock, setting, true)
	if buy.Commission != 23_540 || buy.StampDuty != 0 || buy.TransferFee != 1_000 || buy.Total != 24_540 {
		t.Fatalf("shanghai buy fee = %+v", buy)
	}
	sell := ComputeOrderFee([]int64{100_000_000}, InstrumentShanghaiStock, setting, false)
	if sell.Commission != 23_540 || sell.StampDuty != 50_000 || sell.TransferFee != 1_000 || sell.Total != 74_540 {
		t.Fatalf("shanghai sell fee = %+v", sell)
	}

	// 两笔几乎相同的卖出：印花税 / 过户费逐笔取整再相加。
	fee := ComputeOrderFee([]int64{366_100, 366_700}, InstrumentShanghaiStock, setting, false)
	if fee.Commission != 500 || fee.StampDuty != 366 || fee.TransferFee != 8 || fee.Total != 874 {
		t.Fatalf("two-fill sell fee = %+v", fee)
	}
	allocated := AllocateOrderFee(fee, []int64{366_100, 366_700}, InstrumentShanghaiStock, setting, false)
	var sum FeeBreakdown
	for _, part := range allocated {
		sum.Commission += part.Commission
		sum.StampDuty += part.StampDuty
		sum.TransferFee += part.TransferFee
		sum.Total += part.Total
	}
	if sum != fee {
		t.Fatalf("allocated sum = %+v, want %+v", sum, fee)
	}

	if got := AllocateByAmount(100, []int64{1, 1, 1}); got[0] != 33 || got[1] != 33 || got[2] != 34 {
		t.Fatalf("AllocateByAmount = %v", got)
	}

	fund := ComputeOrderFee([]int64{10_000}, InstrumentShanghaiFund, setting, false)
	if fund.StampDuty != 0 || fund.TransferFee != 0 || fund.Commission != 500 {
		t.Fatalf("fund fee = %+v", fund)
	}
}

func TestStockFeeValidation(t *testing.T) {
	if err := ValidateStockFeeSetting(DefaultStockFeeSetting()); err != nil {
		t.Fatalf("default fee invalid: %v", err)
	}
	bad := DefaultStockFeeSetting()
	bad.CommissionRate = 0
	if err := ValidateStockFeeSetting(bad); err == nil {
		t.Fatal("zero commission accepted")
	}
	bad = DefaultStockFeeSetting()
	bad.TransferFeeRate = -1
	if err := ValidateStockFeeSetting(bad); err == nil {
		t.Fatal("negative transfer fee accepted")
	}
	round := RoundFeeOfSetting(DefaultStockFeeSetting())
	if err := ValidateRoundFee(round); err != nil {
		t.Fatalf("round fee invalid: %v", err)
	}
	if SettingOfRoundFee(round).CommissionRate != round.CommissionRate {
		t.Fatal("round fee round-trip changed the rate")
	}
}
