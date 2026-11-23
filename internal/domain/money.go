package domain

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// 金额换算：金额一律以整数「分」存储，界面层负责分/元换算；股票价格是另一套单位，
// 恒为整数「厘」（1/1000 元）。两者只在计算成交额时相遇一次。
//
// 这些函数是「金额恒为整数分」纪律的守门人，行为是硬契约（含负号、".5"、
// 第三位小数四舍五入这类输入），不要凭感觉改写。

var (
	// ErrInvalidMoney 表示输入不是金额文本。
	ErrInvalidMoney = errors.New("金额格式不正确")
	// ErrMoneyOverflow 表示金额超出 int64 的可表示范围。
	ErrMoneyOverflow = errors.New("金额超出范围")
)

// CentsToYuan 把分格式化成元字符串：固定两位小数，不做千分位分组。
func CentsToYuan(cents int64) string {
	body, negative := magnitudeAndSign(cents)
	whole, fraction := body/100, body%100
	return signPrefix(negative) + strconv.FormatUint(whole, 10) + "." + pad2(fraction)
}

// YuanToCents 把界面输入框里的元文本转成分。
//
// 允许前后空格与负号；整数部分可省略（".5"）；小数最多取 3 位，第 3 位四舍五入到分；
// 空串与纯空白按 0 处理。非数字输入返回 ErrInvalidMoney，
// 超出 int64 范围返回 ErrMoneyOverflow。
func YuanToCents(input string) (int64, error) {
	trimmed := strings.TrimSpace(input)
	negative := strings.HasPrefix(trimmed, "-")
	body := trimmed
	if negative {
		body = trimmed[1:]
	}

	integerRaw, decimalsRaw, _ := strings.Cut(body, ".")
	integerPart := integerRaw
	if integerPart == "" {
		integerPart = "0"
	}
	if !asciiDigits(integerPart) || (decimalsRaw != "" && !asciiDigits(decimalsRaw)) {
		return 0, ErrInvalidMoney
	}

	// 只取到第三位小数；没有的小数位按 0 补。
	digits := [3]byte{'0', '0', '0'}
	copy(digits[:], decimalsRaw)
	cents := int64(digits[0]-'0')*10 + int64(digits[1]-'0')
	if digits[2] >= '5' {
		cents++
	}

	integer, err := strconv.ParseInt(integerPart, 10, 64)
	if err != nil {
		return 0, ErrMoneyOverflow
	}
	if integer > (maxInt64-cents)/100 {
		return 0, ErrMoneyOverflow
	}
	total := integer*100 + cents
	if total < 0 {
		return 0, ErrMoneyOverflow
	}
	if negative {
		return -total, nil
	}
	return total, nil
}

// MilliToYuan 把厘格式化成价格字符串：能整除分时两位小数，带厘位时三位小数，
// 照实反映存下来的精度。
func MilliToYuan(milli int64) string {
	if milli%10 == 0 {
		return CentsToYuan(milli / 10)
	}
	body, negative := magnitudeAndSign(milli)
	whole, fraction := body/1000, body%1000
	return signPrefix(negative) + strconv.FormatUint(whole, 10) + "." + pad3(fraction)
}

// PriceYuanToMilli 把浮点价格（元）转成整数厘，四舍五入到整数厘。
func PriceYuanToMilli(priceYuan float64) int64 {
	return int64(RoundHalfAwayFromZero(priceYuan * 1000))
}

// MilliToPriceYuan 是 PriceYuanToMilli 的逆：整数厘转成浮点价格（元）。
func MilliToPriceYuan(milli int64) float64 {
	return float64(milli) / 1000
}

// RoundHalfAwayFromZero 是全局统一的取整口径：四舍五入，半值远离零。
// Go 的 math.Round 已经是这个语义，这里显式命名，避免有人换成向 +∞ 取整。
func RoundHalfAwayFromZero(value float64) float64 {
	return math.Round(value)
}

const maxInt64 = int64(^uint64(0) >> 1)

// magnitudeAndSign 返回 |value| 的无符号值和是否为负。MinInt64 取绝对值会溢出
// int64，所以先转无符号再补 1。
func magnitudeAndSign(value int64) (uint64, bool) {
	if value >= 0 {
		return uint64(value), false
	}
	return uint64(-(value + 1)) + 1, true
}

func signPrefix(negative bool) string {
	if negative {
		return "-"
	}
	return ""
}

func pad2(value uint64) string {
	if value < 10 {
		return "0" + strconv.FormatUint(value, 10)
	}
	return strconv.FormatUint(value, 10)
}

func pad3(value uint64) string {
	switch {
	case value < 10:
		return "00" + strconv.FormatUint(value, 10)
	case value < 100:
		return "0" + strconv.FormatUint(value, 10)
	default:
		return strconv.FormatUint(value, 10)
	}
}

func asciiDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
