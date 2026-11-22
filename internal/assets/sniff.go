package assets

import (
	"encoding/binary"
	"strings"
)

// Format is an image format recognized by its bytes. The name is the MIME type
// the old application used; the extension and whether the application can draw
// it follow from it.
type Format string

const (
	FormatJPEG Format = "image/jpeg"
	FormatPNG  Format = "image/png"
	FormatGIF  Format = "image/gif"
	FormatWebP Format = "image/webp"
	FormatHEIC Format = "image/heic"
)

// Extension is the file extension the format is stored under. HEIC has one even
// though this application cannot decode it: old workspaces may still hold the
// original file next to its JPEG display copy.
func (format Format) Extension() string {
	switch format {
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	case FormatGIF:
		return "gif"
	case FormatWebP:
		return "webp"
	case FormatHEIC:
		return "heic"
	}
	return ""
}

// Displayable reports whether this application has a decoder for the format.
// HEIC is decoded in-process by github.com/gen2brain/h265/heic, so it is
// displayable too; Save still refuses it as the stored display copy because
// every paint should read a cheap JPEG, not run the HEVC decoder again.
func (format Format) Displayable() bool {
	switch format {
	case FormatJPEG, FormatPNG, FormatGIF, FormatWebP, FormatHEIC:
		return true
	}
	return false
}

// Sniff reads the format from the first bytes, never from a file name: a user
// can rename anything, and a workspace must not keep a file the application
// cannot draw. The empty Format means unrecognized (or AVIF, which this
// application cannot decode either).
func Sniff(data []byte) Format {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return FormatPNG
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return FormatJPEG
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return FormatGIF
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return FormatWebP
	case isHeic(data):
		return FormatHEIC
	}
	return ""
}

// heifBrands are the ISO BMFF brands that mean a HEVC-coded still. AVIF uses
// the same ftyp box but a codec this application cannot decode either; it is
// left out so that only a real HEIC file gets the HEIC sentence.
var heifBrands = map[string]bool{
	"heic": true, "heix": true, "heim": true, "heis": true,
	"hevc": true, "hevx": true, "hevm": true, "hevs": true,
	"mif1": true, "msf1": true, "mif2": true,
}

var avifBrands = map[string]bool{"avif": true, "avis": true}

// isHeic reports whether the bytes are an ISO BMFF HEIC/HEIF still. The old
// application's shared/domain/image-signature.ts did the same check.
func isHeic(data []byte) bool {
	brands := ftypBrands(data)
	if len(brands) == 0 {
		return false
	}
	heic, avif := false, false
	for _, brand := range brands {
		if avifBrands[brand] {
			avif = true
		}
		if heifBrands[brand] {
			heic = true
		}
	}
	return heic && !avif
}

// ftypBrands reads every brand in the file's ftyp box: the major brand and the
// compatible ones that follow it.
func ftypBrands(data []byte) []string {
	if len(data) < 16 || string(data[4:8]) != "ftyp" {
		return nil
	}
	// The box is small; a caller may hand in a whole photo, so look at the
	// declared length and cap the scan.
	end := len(data)
	if declared := int(binary.BigEndian.Uint32(data[:4])); declared > 8 && declared < end {
		end = declared
	}
	if end > 256 {
		end = 256
	}
	var brands []string
	for offset := 8; offset+4 <= end; offset += 4 {
		brands = append(brands, string(data[offset:offset+4]))
	}
	return brands
}

// formatExtension maps an extension to a format, for classifying picked files
// that might be a live photo's video rather than a still. The still's real
// format still comes from Sniff.
func isMotionExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case "mov", "mp4", "m4v", "webm":
		return true
	}
	return false
}
