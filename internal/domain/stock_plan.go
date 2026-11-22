package domain

import (
	"math"
	"strconv"
	"strings"
)

// PlanRatioKind says why the risk/reward ratio has no number: the two levels
// are not both written, are not readable as prices, or point the wrong way.
type PlanRatioKind uint8

const (
	PlanRatioMissing PlanRatioKind = iota
	PlanRatioUnreadable
	PlanRatioBlocked
	PlanRatioValue
)

// PlanLevels are the levels a buy plan implies: where the stop and the target
// sit relative to the average buy price, and the risk/reward between them.
// The ledger stores the user's own text; this is derived for display only.
type PlanLevels struct {
	// Upside and Downside are signed percentages such as "+6.53%" and
	// "-4.86%"; empty when that level is not a readable price.
	Upside   string
	Downside string
	// RatioKind and RatioText describe the risk/reward: (target - buy) /
	// (buy - stop). RatioText is set only for PlanRatioValue.
	RatioKind PlanRatioKind
	RatioText string
}

// PlanLevelsOf derives the plan levels from the ledger's buy price and the
// user's stop-loss / take-profit text. A level that is not a plain price is
// ignored for its own percentage; the ratio then reports why it cannot be
// computed instead of pretending the field is empty.
func PlanLevelsOf(buyPrice, stopLoss, takeProfit string) PlanLevels {
	cost, ok := yuanTextMilli(buyPrice)
	if !ok || cost <= 0 {
		return PlanLevels{RatioKind: PlanRatioMissing}
	}
	take, takeOK := yuanTextMilli(takeProfit)
	stop, stopOK := yuanTextMilli(stopLoss)
	levels := PlanLevels{}
	if takeOK {
		levels.Upside = planPercent(take, cost)
	}
	if stopOK {
		levels.Downside = planPercent(stop, cost)
	}
	switch {
	case strings.TrimSpace(stopLoss) == "" || strings.TrimSpace(takeProfit) == "":
		levels.RatioKind = PlanRatioMissing
	case !stopOK || !takeOK:
		levels.RatioKind = PlanRatioUnreadable
	case take <= cost || stop >= cost:
		levels.RatioKind = PlanRatioBlocked
	default:
		levels.RatioKind = PlanRatioValue
		levels.RatioText = planRatioText(take, cost, stop)
	}
	return levels
}

// yuanTextMilli parses a plain yuan price with at most three decimals, the
// precision prices are stored at. A leading "+" and surrounding spaces are
// accepted; anything else is not a price.
func yuanTextMilli(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "+")
	if text == "" {
		return 0, false
	}
	integerText, fractionText, hasFraction := strings.Cut(text, ".")
	if integerText == "" || (hasFraction && len(fractionText) > 3) {
		return 0, false
	}
	for _, r := range integerText {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	for _, r := range fractionText {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	integer, err := strconv.ParseInt(integerText, 10, 64)
	if err != nil || integer > (math.MaxInt64-999)/1000 {
		return 0, false
	}
	milli := integer * 1000
	if hasFraction && fractionText != "" {
		fraction, err := strconv.ParseInt(fractionText, 10, 64)
		if err != nil {
			return 0, false
		}
		for index := len(fractionText); index < 3; index++ {
			fraction *= 10
		}
		milli += fraction
	}
	return milli, true
}

// planPercent is level relative to cost as a signed two-decimal percentage.
func planPercent(level, cost int64) string {
	hundredths := int64(RoundHalfAwayFromZero(float64(level-cost) * 10_000 / float64(cost)))
	if hundredths == 0 {
		return "0.00%"
	}
	sign := "+"
	if hundredths < 0 {
		sign = "-"
		hundredths = -hundredths
	}
	return sign + strconv.FormatInt(hundredths/100, 10) + "." +
		pad2(uint64(hundredths%100)) + "%"
}

func planRatioText(take, cost, stop int64) string {
	hundredths := int64(RoundHalfAwayFromZero(float64(take-cost) * 100 / float64(cost-stop)))
	return strconv.FormatInt(hundredths/100, 10) + "." + pad2(uint64(hundredths%100))
}
