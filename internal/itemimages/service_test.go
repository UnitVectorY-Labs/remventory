package itemimages

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"testing"
)

type memoryFile struct{ *bytes.Reader }

func (memoryFile) Close() error { return nil }

func TestPrepareCreatesSquareCenterCroppedThumbnail(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 600, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 600; x++ {
			pixel := color.RGBA{R: 240, A: 255}
			if x >= 300 {
				pixel = color.RGBA{B: 240, A: 255}
			}
			source.Set(x, y, pixel)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	prepared, err := Prepare(memoryFile{bytes.NewReader(encoded.Bytes())}, &multipart.FileHeader{Filename: "pattern.png"})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Width != 600 || prepared.Height != 300 || prepared.MIMEType != "image/png" {
		t.Fatalf("unexpected metadata: %#v", prepared)
	}
	thumbnail, err := jpeg.Decode(bytes.NewReader(prepared.Thumbnail))
	if err != nil {
		t.Fatal(err)
	}
	if thumbnail.Bounds().Dx() != ThumbnailSize || thumbnail.Bounds().Dy() != ThumbnailSize {
		t.Fatalf("thumbnail bounds = %v", thumbnail.Bounds())
	}
	leftR, _, leftB, _ := thumbnail.At(10, 160).RGBA()
	rightR, _, rightB, _ := thumbnail.At(310, 160).RGBA()
	if leftR <= leftB || rightB <= rightR {
		t.Fatal("thumbnail did not retain the expected centered red/blue crop")
	}
}

type fakeObjects struct {
	putKeys    []string
	deletedKey string
	failAt     int
}

func (f *fakeObjects) Put(_ context.Context, key, _ string, _ []byte) error {
	f.putKeys = append(f.putKeys, key)
	if len(f.putKeys) == f.failAt {
		return errors.New("put failed")
	}
	return nil
}

func (*fakeObjects) Get(context.Context, string) (Object, error) {
	return Object{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (f *fakeObjects) Delete(_ context.Context, key string) error { f.deletedKey = key; return nil }

func TestStorePreparedCleansUpOriginalWhenThumbnailFails(t *testing.T) {
	objects := &fakeObjects{failAt: 2}
	prepared := Prepared{OriginalKey: "original", ThumbnailKey: "thumbnail", MIMEType: "image/png"}
	if err := StorePrepared(context.Background(), objects, prepared); err == nil {
		t.Fatal("expected thumbnail upload error")
	}
	if objects.deletedKey != "original" {
		t.Fatalf("deleted key = %q", objects.deletedKey)
	}
}
