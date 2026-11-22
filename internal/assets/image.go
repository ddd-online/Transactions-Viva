package assets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"

	"github.com/vivaui/viva/gfx"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// Decode turns image bytes into a picture this application can draw. JPEG's
// EXIF orientation is applied: a phone photo is stored the way its camera saw
// it, not sideways. HEIC is refused with a sentence, because the platform's
// decoders are not available to this Go process.
func Decode(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("图片内容为空")
	}
	if len(data) > MaxImageBytes {
		return nil, fmt.Errorf("图片过大（超过 %d MB）", MaxImageBytes>>20)
	}
	format := Sniff(data)
	switch format {
	case FormatHEIC:
		return nil, fmt.Errorf("HEIC 图片暂不支持，请先转换为 JPEG 后添加")
	case "":
		return nil, fmt.Errorf("图片不是可解码的图片（支持 JPEG / PNG / GIF / WebP）")
	}
	config, err := ImageConfig(data)
	if err != nil {
		return nil, fmt.Errorf("图片无法解码：%w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("图片尺寸无效")
	}
	if int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return nil, fmt.Errorf("图片尺寸过大（超过 %d 万像素）", MaxImagePixels/10_000)
	}

	var picture image.Image
	switch format {
	case FormatJPEG:
		picture, err = jpeg.Decode(bytes.NewReader(data))
	case FormatPNG:
		picture, err = png.Decode(bytes.NewReader(data))
	case FormatGIF:
		// A GIF's animation has no place in a still gallery; its first frame
		// is what the old browser <img> would show before playing it.
		picture, err = gif.Decode(bytes.NewReader(data))
	case FormatWebP:
		picture, err = webp.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("图片无法解码：%w", err)
	}
	rgba := toRGBA(picture)
	if format == FormatJPEG {
		rgba = applyOrientation(rgba, exifOrientation(data))
	}
	return rgba, nil
}

// ImageConfig reads the dimensions without decoding the pixels, so a huge
// picture is refused before it can exhaust memory.
func ImageConfig(data []byte) (image.Config, error) {
	switch Sniff(data) {
	case FormatJPEG:
		return jpeg.DecodeConfig(bytes.NewReader(data))
	case FormatPNG:
		return png.DecodeConfig(bytes.NewReader(data))
	case FormatGIF:
		return gif.DecodeConfig(bytes.NewReader(data))
	case FormatWebP:
		return webp.DecodeConfig(bytes.NewReader(data))
	}
	return image.Config{}, fmt.Errorf("不支持的图片格式")
}

// Thumbnail makes the list thumbnail: JPEG, at most maxWidth wide, never
// upscaled, over an opaque white background so a transparent PNG does not turn
// black in a format without alpha.
func Thumbnail(picture image.Image, maxWidth int) ([]byte, error) {
	bounds := picture.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, fmt.Errorf("图片尺寸无效")
	}
	width := bounds.Dx()
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	height := int(math.Round(float64(bounds.Dy()) * float64(width) / float64(bounds.Dx())))
	height = max(height, 1)
	scaled := toRGBA(picture)
	if scaled.Rect.Dx() != width || scaled.Rect.Dy() != height {
		scaled = resize(scaled, width, height)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	stddraw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	stddraw.Draw(canvas, canvas.Bounds(), scaled, image.Point{}, stddraw.Over)
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: ThumbnailQuality}); err != nil {
		return nil, fmt.Errorf("生成缩略图失败：%w", err)
	}
	return buffer.Bytes(), nil
}

// Load reads one stored image and returns the bitmap a page paints. A picture
// longer than maxEdge on its longest side is downscaled first: the window never
// shows more pixels than that, and keeping a 12 megapixel photo for a 600 pixel
// pane wastes memory for as long as the page lives.
func Load(workspaceDir, relative string, maxEdge int) (gfx.Bitmap, error) {
	full, err := Resolve(workspaceDir, relative)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return gfx.Bitmap{}, fmt.Errorf("读取图片失败：%w", err)
	}
	if info.Size() > MaxImageBytes {
		return gfx.Bitmap{}, fmt.Errorf("图片过大（超过 %d MB）", MaxImageBytes>>20)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return gfx.Bitmap{}, fmt.Errorf("读取图片失败：%w", err)
	}
	picture, err := Decode(data)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	return Bitmap(fitEdge(picture, maxEdge)), nil
}

// Bitmap converts a decoded picture into the canvas's immutable premultiplied
// BGRA pixels.
func Bitmap(picture image.Image) gfx.Bitmap {
	rgba := toRGBA(picture)
	width, height := rgba.Rect.Dx(), rgba.Rect.Dy()
	pixels := make([]byte, width*height*4)
	for y := range height {
		source := rgba.Pix[y*rgba.Stride:]
		target := pixels[y*width*4:]
		for x := range width {
			s := source[x*4 : x*4+4]
			d := target[x*4 : x*4+4]
			d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
		}
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4}
}

// toRGBA copies any decoded image into a tightly-packed premultiplied RGBA
// picture, the shape the conversions and the EXIF rotation work on.
func toRGBA(picture image.Image) *image.RGBA {
	bounds := picture.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stddraw.Draw(rgba, rgba.Bounds(), picture, bounds.Min, stddraw.Src)
	return rgba
}

// resize scales a picture to exactly width × height.
func resize(picture *image.RGBA, width, height int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(target, target.Bounds(), picture, picture.Bounds(), xdraw.Src, nil)
	return target
}

// fitEdge scales a picture down until its longest edge is at most maxEdge. A
// picture that already fits is returned unchanged.
func fitEdge(picture image.Image, maxEdge int) image.Image {
	if maxEdge <= 0 {
		return picture
	}
	bounds := picture.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxEdge && height <= maxEdge {
		return picture
	}
	scale := float64(maxEdge) / float64(max(width, height))
	newWidth := max(int(math.Round(float64(width)*scale)), 1)
	newHeight := max(int(math.Round(float64(height)*scale)), 1)
	return resize(toRGBA(picture), newWidth, newHeight)
}

// applyOrientation turns a decoded JPEG according to its EXIF orientation. The
// values are the eight tags cameras write; anything else leaves the picture as
// decoded.
func applyOrientation(source *image.RGBA, orientation int) *image.RGBA {
	if orientation < 2 || orientation > 8 {
		return source
	}
	width, height := source.Rect.Dx(), source.Rect.Dy()
	targetWidth, targetHeight := width, height
	if orientation >= 5 {
		targetWidth, targetHeight = height, width
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := range targetHeight {
		for x := range targetWidth {
			sx, sy := x, y
			switch orientation {
			case 2: // mirror horizontal
				sx = width - 1 - x
			case 3: // rotate 180
				sx, sy = width-1-x, height-1-y
			case 4: // mirror vertical
				sy = height - 1 - y
			case 5: // transpose
				sx, sy = y, x
			case 6: // rotate 90 clockwise
				sx, sy = y, height-1-x
			case 7: // transverse
				sx, sy = width-1-y, height-1-x
			case 8: // rotate 90 counter-clockwise
				sx, sy = width-1-y, x
			}
			copy(target.Pix[y*target.Stride+x*4:y*target.Stride+x*4+4],
				source.Pix[sy*source.Stride+sx*4:sy*source.Stride+sx*4+4])
		}
	}
	return target
}

// exifOrientation reads tag 0x0112 from a JPEG's APP1 Exif segment. 1 is the
// default (normal), as is a file without the tag.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	offset := 2
	for offset+4 <= len(data) {
		if data[offset] != 0xff {
			return 1
		}
		marker := data[offset+1]
		switch {
		case marker == 0xd8 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7):
			offset += 2
			continue
		case marker == 0xda || marker == 0xd9:
			return 1
		}
		length := int(binary.BigEndian.Uint16(data[offset+2:]))
		if length < 2 || offset+2+length > len(data) {
			return 1
		}
		segment := data[offset+4 : offset+2+length]
		if marker == 0xe1 && len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
			if orientation := tiffOrientation(segment[6:]); orientation != 0 {
				return orientation
			}
		}
		offset += 2 + length
	}
	return 1
}

// tiffOrientation reads the orientation tag out of the TIFF header an Exif
// segment carries.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}
	directory := int(order.Uint32(tiff[4:8]))
	if directory < 8 || directory+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[directory:]))
	for index := range count {
		entry := directory + 2 + index*12
		if entry+12 > len(tiff) {
			return 0
		}
		tag := order.Uint16(tiff[entry:])
		kind := order.Uint16(tiff[entry+2:])
		number := order.Uint32(tiff[entry+4:])
		if tag != 0x0112 || kind != 3 || number < 1 {
			continue
		}
		orientation := int(order.Uint16(tiff[entry+8:]))
		if orientation >= 1 && orientation <= 8 {
			return orientation
		}
		return 0
	}
	return 0
}
