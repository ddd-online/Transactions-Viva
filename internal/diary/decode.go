package diary

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// DecodeText reads a diary file's bytes. The order is the service's, and it
// matters: a file that is valid UTF-8 is taken as it is (a UTF-8 byte-order mark
// included, so an exported file re-imports byte for byte), then a UTF-16 file
// with a mark, then GBK, and only then is a file whose bytes are not any of those
// decoded with every invalid byte replaced by `?`.
func DecodeText(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	if decoded, ok := decodeUTF16(raw); ok {
		return decoded
	}
	// The GBK decoder replaces bytes it cannot read instead of failing, so a
	// result with a replacement character means this was not GBK either: a real
	// GBK text cannot contain U+FFFD.
	if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw); err == nil && !bytes.ContainsRune(decoded, '\uFFFD') {
		return string(decoded)
	}
	return replaceInvalidUTF8(raw)
}

// decodeUTF16 reads a UTF-16 file that starts with a byte-order mark, little or
// big endian. It reports false when there is no mark: without one, UTF-16 is a
// guess, and GBK is the likelier text for a file that is not UTF-8.
func decodeUTF16(raw []byte) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	var littleEndian bool
	switch {
	case raw[0] == 0xff && raw[1] == 0xfe:
		littleEndian = true
	case raw[0] == 0xfe && raw[1] == 0xff:
		littleEndian = false
	default:
		return "", false
	}
	body := raw[2:]
	// An odd trailing byte has no partner; it is dropped, as the decoder's
	// two-byte chunks did.
	body = body[:len(body)-len(body)%2]
	units := make([]uint16, len(body)/2)
	for index := range units {
		if littleEndian {
			units[index] = uint16(body[2*index]) | uint16(body[2*index+1])<<8
		} else {
			units[index] = uint16(body[2*index])<<8 | uint16(body[2*index+1])
		}
	}
	return string(utf16.Decode(units)), true
}

// utf8Shape is the shape of a multi-byte UTF-8 sequence: its length and the range
// the second byte may take. The rest of the continuation bytes are always
// 0x80–0xBF.
type utf8Shape struct {
	length    int
	secondMin byte
	secondMax byte
}

// shapeOf maps a leading byte to its sequence shape; nil when the byte cannot
// start a sequence at all (a stray continuation byte, or the five- and six-byte
// forms UTF-8 no longer allows).
func shapeOf(first byte) *utf8Shape {
	switch {
	case first >= 0xc2 && first <= 0xdf:
		return &utf8Shape{2, 0x80, 0xbf}
	case first == 0xe0:
		return &utf8Shape{3, 0xa0, 0xbf}
	case first >= 0xe1 && first <= 0xec:
		return &utf8Shape{3, 0x80, 0xbf}
	case first == 0xed:
		return &utf8Shape{3, 0x80, 0x9f}
	case first >= 0xee && first <= 0xef:
		return &utf8Shape{3, 0x80, 0xbf}
	case first == 0xf0:
		return &utf8Shape{4, 0x90, 0xbf}
	case first >= 0xf1 && first <= 0xf3:
		return &utf8Shape{4, 0x80, 0xbf}
	case first == 0xf4:
		return &utf8Shape{4, 0x80, 0x8f}
	}
	return nil
}

// replaceInvalidUTF8 decodes what it can and writes `?` for each sequence that is
// not valid, so a partly corrupt file still opens with its readable text in
// place. A sequence is abandoned at the byte that broke it.
func replaceInvalidUTF8(raw []byte) string {
	output := make([]rune, 0, len(raw))
	for index := 0; index < len(raw); {
		first := raw[index]
		if first < 0x80 {
			output = append(output, rune(first))
			index++
			continue
		}
		shape := shapeOf(first)
		if shape == nil {
			output = append(output, '?')
			index++
			continue
		}
		skip := 0
		for offset := 1; offset < shape.length; offset++ {
			if index+offset >= len(raw) {
				// The file ends inside a character: one byte is skipped.
				skip = 1
				break
			}
			next := raw[index+offset]
			min, max := byte(0x80), byte(0xbf)
			if offset == 1 {
				min, max = shape.secondMin, shape.secondMax
			}
			if next < min || next > max {
				skip = offset
				break
			}
		}
		if skip > 0 {
			output = append(output, '?')
			index += skip
			continue
		}
		value, _ := utf8.DecodeRune(raw[index:])
		output = append(output, value)
		index += shape.length
	}
	return string(output)
}
