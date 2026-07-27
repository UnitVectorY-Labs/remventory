package itemimages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

const (
	MaxUploadBytes = 12 << 20
	ThumbnailSize  = 320
	maxImagePixels = 50_000_000
)

var ErrTooLarge = errors.New("image exceeds the 12 MB upload limit")

type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

type ObjectStore interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string) (Object, error)
	Delete(context.Context, string) error
}

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(ctx context.Context, cfg config.Config) (*S3Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(strings.TrimRight(cfg.S3Endpoint, "/"))
		options.UsePathStyle = cfg.S3UsePathStyle
	})
	return &S3Store{client: client, bucket: cfg.S3Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) (Object, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return Object{}, err
	}
	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}
	return Object{Body: result.Body, ContentType: contentType, Size: aws.ToInt64(result.ContentLength)}, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

type Prepared struct {
	ID               string
	OriginalKey      string
	ThumbnailKey     string
	Original         []byte
	Thumbnail        []byte
	MIMEType         string
	OriginalFilename string
	Width            int
	Height           int
}

func Prepare(file multipart.File, header *multipart.FileHeader) (Prepared, error) {
	limited := io.LimitReader(file, MaxUploadBytes+1)
	original, err := io.ReadAll(limited)
	if err != nil {
		return Prepared{}, fmt.Errorf("read image: %w", err)
	}
	if len(original) > MaxUploadBytes {
		return Prepared{}, ErrTooLarge
	}
	if len(original) == 0 {
		return Prepared{}, errors.New("image is empty")
	}

	decodedConfig, format, err := image.DecodeConfig(bytes.NewReader(original))
	if err != nil {
		return Prepared{}, errors.New("file is not a supported JPEG, PNG, or GIF image")
	}
	if decodedConfig.Width <= 0 || decodedConfig.Height <= 0 || int64(decodedConfig.Width)*int64(decodedConfig.Height) > maxImagePixels {
		return Prepared{}, errors.New("image dimensions are invalid or too large")
	}
	decoded, _, err := image.Decode(bytes.NewReader(original))
	if err != nil {
		return Prepared{}, errors.New("image could not be decoded")
	}

	mimeType := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "gif": "image/gif"}[format]
	if mimeType == "" || !strings.HasPrefix(http.DetectContentType(original), "image/") {
		return Prepared{}, errors.New("unsupported image type")
	}
	thumbnail, err := squareThumbnail(decoded, ThumbnailSize)
	if err != nil {
		return Prepared{}, err
	}

	id := uuid.NewString()
	filename := filepath.Base(header.Filename)
	return Prepared{
		ID: id, OriginalKey: "items/" + id + "/original", ThumbnailKey: "items/" + id + "/thumbnail.jpg",
		Original: original, Thumbnail: thumbnail, MIMEType: mimeType, OriginalFilename: filename,
		Width: decodedConfig.Width, Height: decodedConfig.Height,
	}, nil
}

func squareThumbnail(source image.Image, size int) ([]byte, error) {
	bounds := source.Bounds()
	side := bounds.Dx()
	if bounds.Dy() < side {
		side = bounds.Dy()
	}
	left := bounds.Min.X + (bounds.Dx()-side)/2
	top := bounds.Min.Y + (bounds.Dy()-side)/2
	cropped := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(cropped, cropped.Bounds(), source, image.Pt(left, top), draw.Src)
	thumbnail := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, thumbnail, &jpeg.Options{Quality: 84}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return output.Bytes(), nil
}

func StorePrepared(ctx context.Context, objects ObjectStore, prepared Prepared) error {
	if err := objects.Put(ctx, prepared.OriginalKey, prepared.MIMEType, prepared.Original); err != nil {
		return fmt.Errorf("store original image: %w", err)
	}
	if err := objects.Put(ctx, prepared.ThumbnailKey, "image/jpeg", prepared.Thumbnail); err != nil {
		_ = objects.Delete(ctx, prepared.OriginalKey)
		return fmt.Errorf("store thumbnail: %w", err)
	}
	return nil
}
