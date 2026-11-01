package itemimages

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"testing"
)

func orientedJPEG(t *testing.T, orientation uint16, order binary.ByteOrder) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 80, 60))
	colors := []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}}
	for y := 0; y < 60; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, colors[(y/30)*2+x/40])
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	exif := make([]byte, 32)
	copy(exif, "Exif\x00\x00")
	tiff := exif[6:]
	copy(tiff, "II")
	if order == binary.BigEndian {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], orientation)
	data := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	return append(data, encoded.Bytes()[2:]...)
}

func cornerColors(img image.Image) []int {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	result := []int{}
	for _, p := range []image.Point{{w / 4, h / 4}, {3 * w / 4, h / 4}, {w / 4, 3 * h / 4}, {3 * w / 4, 3 * h / 4}} {
		r, g, b, _ := img.At(p.X, p.Y).RGBA()
		value := 2
		if r > b && g > b && r > 30000 && g > 30000 {
			value = 3
		} else if r > g && r > b {
			value = 0
		} else if g > r && g > b {
			value = 1
		}
		result = append(result, value)
	}
	return result
}

func TestCameraOrientationMatchesPreviewInThumbnailAndAnalysis(t *testing.T) {
	want := [][]int{{0, 1, 2, 3}, {1, 0, 3, 2}, {3, 2, 1, 0}, {2, 3, 0, 1}, {0, 2, 1, 3}, {2, 0, 3, 1}, {3, 1, 2, 0}, {1, 3, 0, 2}}
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := uint16(1); orientation <= 8; orientation++ {
			t.Run(fmt.Sprintf("%T/%d", order, orientation), func(t *testing.T) {
				data := orientedJPEG(t, orientation, order)
				prepared, err := Prepare(memoryFile{bytes.NewReader(data)}, &multipart.FileHeader{Filename: "phone.jpg"})
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(prepared.Original, data) {
					t.Fatal("original changed")
				}
				width, height := 80, 60
				if orientation >= 5 {
					width, height = height, width
				}
				if prepared.Width != width || prepared.Height != height {
					t.Fatal("incorrect displayed dimensions")
				}
				analysis, err := AnalysisImage(data)
				if err != nil {
					t.Fatal(err)
				}
				for _, encoded := range [][]byte{prepared.Thumbnail, analysis} {
					img, err := jpeg.Decode(bytes.NewReader(encoded))
					if err != nil {
						t.Fatal(err)
					}
					if fmt.Sprint(cornerColors(img)) != fmt.Sprint(want[orientation-1]) {
						t.Fatalf("orientation %d: got corners %v, want %v", orientation, cornerColors(img), want[orientation-1])
					}
				}
			})
		}
	}
}

func TestMalformedOrientationMetadataLeavesPixelsUnchanged(t *testing.T) {
	for _, data := range [][]byte{nil, {0xff, 0xd8, 0xff, 0xe1, 0, 1}, {0xff, 0xd8, 0xff, 0xe1, 255, 255}, []byte("Exif\x00\x00")} {
		if jpegOrientation(data) != 1 {
			t.Fatal("invalid metadata changed orientation")
		}
	}
	for n := 0; n < 40; n++ {
		if tiffOrientation(bytes.Repeat([]byte{255}, n)) != 1 {
			t.Fatal("invalid TIFF accepted")
		}
	}
}

type thumbnailObjects struct {
	data  map[string][]byte
	reads map[string]int
	puts  int
}

func (s *thumbnailObjects) Get(_ context.Context, key string) (Object, error) {
	s.reads[key]++
	data, ok := s.data[key]
	if !ok {
		return Object{}, fmt.Errorf("missing object")
	}
	return Object{Body: io.NopCloser(bytes.NewReader(data)), ContentType: "image/jpeg", Size: int64(len(data))}, nil
}
func (s *thumbnailObjects) Put(_ context.Context, key, _ string, data []byte) error {
	s.puts++
	s.data[key] = data
	return nil
}
func (s *thumbnailObjects) Delete(_ context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func TestLegacyThumbnailIsCorrectedAndCachedWithoutChangingOriginal(t *testing.T) {
	original := orientedJPEG(t, 6, binary.LittleEndian)
	objects := &thumbnailObjects{data: map[string][]byte{"items/id/original": original, "items/id/thumbnail.jpg": []byte("old sideways thumbnail")}, reads: map[string]int{}}
	for i := 0; i < 2; i++ {
		object, err := LoadThumbnail(context.Background(), objects, "items/id/original", "items/id/thumbnail.jpg")
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(object.Body)
		object.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(cornerColors(img)) != "[2 0 3 1]" {
			t.Fatal("legacy thumbnail is still sideways")
		}
	}
	if objects.reads["items/id/original"] != 1 || objects.puts != 1 {
		t.Fatal("corrected thumbnail not cached")
	}
	if !bytes.Equal(objects.data["items/id/original"], original) || string(objects.data["items/id/thumbnail.jpg"]) != "old sideways thumbnail" {
		t.Fatal("original objects were overwritten")
	}
}
