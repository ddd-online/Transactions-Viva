package events

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// eventTone is one of the twenty identity colours an event can wear. The
// hexadecimals are content, not chrome: the one the user picks is stored in
// key_event.color, so this list is the authority. Changing a value affects rows
// already carrying it.
type eventTone struct {
	value string
	name  string
}

// eventTones is the palette in its order, including the two colours darkened
// for a 3:1 contrast on paper (#B08534 土黄, #C17C33 橘黄).
var eventTones = [...]eventTone{
	{"#D9705A", "砖红"},
	{"#C25460", "玫红"},
	{"#D07048", "橘红"},
	{"#C17C33", "橘黄"},
	{"#B08534", "土黄"},
	{"#A09040", "橄榄黄"},
	{"#5C9858", "草绿"},
	{"#4A8E70", "松绿"},
	{"#5C9E7C", "豆绿"},
	{"#3D8878", "墨绿"},
	{"#389098", "青蓝"},
	{"#4A78A0", "钢蓝"},
	{"#5C8DB5", "天蓝"},
	{"#6070A0", "靛蓝"},
	{"#7868A0", "蓝紫"},
	{"#8C6B9E", "紫罗兰"},
	{"#A06088", "紫红"},
	{"#B06078", "胭脂"},
	{"#8C7B6E", "茶褐"},
	{"#7E8890", "青灰"},
}

// toneLift is how far a tone is mixed toward white in the dark appearance. The
// stylesheet lifted by 32% in sRGB with color-mix, so the lift happens on
// the 8-bit channels rather than in linear light.
const toneLift = 0.32

// toneColor resolves a stored colour for painting. An empty value means the
// theme's primary (使用默认颜色); an unparseable one (existing data, a hand-edited
// row) falls back to the primary rather than painting nothing.
func toneColor(value string, theme gfx.Theme) gfx.Color {
	red, green, blue, ok := parseHexColor(value)
	if !ok {
		return theme.Primary
	}
	if isDark(theme) {
		red = lift(red)
		green = lift(green)
		blue = lift(blue)
	}
	return gfx.RGB(red, green, blue)
}

// isDark reports whether the theme is the dark appearance. The paper colour is
// the figure the rest of the application compares too.
func isDark(theme gfx.Theme) bool {
	return 0.2126*theme.Background.Red+0.7152*theme.Background.Green+0.0722*theme.Background.Blue < 0.5
}

// lift mixes one 8-bit channel 32% toward white.
func lift(channel uint8) uint8 {
	return uint8(float32(channel)*(1-toneLift) + 255*toneLift + 0.5)
}

// parseHexColor reads "#RRGGBB" (the stored form). The short form is not
// something the palette or the application ever wrote.
func parseHexColor(value string) (uint8, uint8, uint8, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return 0, 0, 0, false
	}
	number, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(number >> 16), uint8(number >> 8), uint8(number), true
}
