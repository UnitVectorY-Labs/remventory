package itemimages

import (
	"bytes"
	"encoding/binary"
	"image"
)

// jpegOrientation reads only the EXIF orientation tag. Invalid metadata leaves
// pixels unchanged, as it does in images without an orientation tag.
func jpegOrientation(data []byte) uint16 {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xff {
			break
		}
		marker := data[pos+1]
		if marker == 0xff {
			pos++
			continue
		}
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			pos += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if size < 2 || size > len(data)-pos-2 {
			break
		}
		segment := data[pos+4 : pos+2+size]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return tiffOrientation(segment[6:])
		}
		pos += 2 + size
	}
	return 1
}

func tiffOrientation(data []byte) uint16 {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset < 8 || offset+2 > uint64(len(data)) {
		return 1
	}
	count := int(order.Uint16(data[offset : offset+2]))
	for pos, i := int(offset)+2, 0; i < count && pos+12 <= len(data); pos, i = pos+12, i+1 {
		entry := data[pos : pos+12]
		if order.Uint16(entry[:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			if value := order.Uint16(entry[8:10]); value >= 1 && value <= 8 {
				return value
			}
		}
	}
	return 1
}

func orient(source image.Image, orientation uint16) image.Image {
	if orientation < 2 || orientation > 8 {
		return source
	}
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	width, height := w, h
	if orientation >= 5 {
		width, height = h, w
	}
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			result.Set(dx, dy, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return result
}
