// Package ids makes the identifiers this application stores. They are RFC 4122
// version 4 UUID strings, the shape the application used for ledgers,
// records, templates, diary books and event images. The store and the event
// image service both name rows, so the generator lives on its own instead of in
// one of them.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// New returns a random RFC 4122 version 4 UUID. It falls back to a
// timestamp-based value only if the platform's random source fails, because an
// empty identifier is worse for data integrity than a predictable one.
func New() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		now := time.Now().UnixNano()
		for index := range bytes {
			bytes[index] = byte(now >> (8 * (index % 8)))
		}
	}
	bytes[6] = bytes[6]&0x0f | 0x40
	bytes[8] = bytes[8]&0x3f | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], bytes[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], bytes[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], bytes[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], bytes[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], bytes[10:16])
	return string(encoded)
}
