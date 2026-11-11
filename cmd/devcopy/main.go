package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"io"
	"os"
)

func client(endpoint, region, key, secret string) *s3.Client {
	return s3.New(s3.Options{Region: region, BaseEndpoint: aws.String(endpoint), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider(key, secret, "")})
}
func main() {
	ctx := context.Background()
	b, err := os.ReadFile(os.ExpandEnv("$HOME/.local/share/remventory-dev/production.json"))
	if err != nil {
		panic("read private production configuration: " + err.Error())
	}
	e := map[string]string{}
	if err = json.Unmarshal(b, &e); err != nil {
		panic("invalid private configuration")
	}
	for _, key := range []string{"S3_ENDPOINT", "S3_REGION", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_BUCKET"} {
		if e[key] == "" {
			panic("missing storage setting: " + key)
		}
	}
	src := client(e["S3_ENDPOINT"], e["S3_REGION"], e["S3_ACCESS_KEY_ID"], e["S3_SECRET_ACCESS_KEY"])
	dst := client("http://127.0.0.1:59000", "garage", "remventorylocalkey", "remventory-local-storage")
	db, err := pgx.Connect(ctx, "postgres://postgres:remventory-local@127.0.0.1:55432/remventory_dev?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close(ctx)
	rows, err := db.Query(ctx, "select object_key,coalesce(thumbnail_object_key,'') from item_assets")
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for rows.Next() {
		var a, b string
		if err = rows.Scan(&a, &b); err != nil {
			panic(err)
		}
		keys = append(keys, a)
		if b != "" {
			keys = append(keys, b)
		}
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
	rows.Close()
	n := 0
	for _, k := range keys {
		obj, err := src.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(e["S3_BUCKET"]), Key: aws.String(k)})
		if err != nil {
			panic("source photo unavailable: " + k)
		}
		data, err := io.ReadAll(obj.Body)
		obj.Body.Close()
		if err != nil {
			panic(err)
		}
		_, err = dst.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String("default"), Key: aws.String(k), ContentType: obj.ContentType, Body: bytes.NewReader(data)})
		if err != nil {
			panic(err)
		}
		n++
	}
	fmt.Println("Copied objects:", n)
	e["DATABASE_URL"] = "postgres://postgres:remventory-local@127.0.0.1:55432/remventory_dev?sslmode=disable"
	e["S3_ENDPOINT"] = "http://127.0.0.1:59000"
	e["S3_REGION"] = "garage"
	e["S3_BUCKET"] = "default"
	e["S3_ACCESS_KEY_ID"] = "remventorylocalkey"
	e["S3_SECRET_ACCESS_KEY"] = "remventory-local-storage"
	e["OPENAI_BASE_URL"] = "https://openrouter.ai/api/v1"
	e["OPENAI_MAIN_MODEL"] = "openai/gpt-6-luna"
	e["OPENAI_VISION_MODEL"] = "openai/gpt-6-luna"
	e["REMVENTORY_HTTP_ADDR"] = "127.0.0.1:58080"
	e["REMVENTORY_WORKER_ENABLED"] = "true"
	b, _ = json.Marshal(e)
	if err = os.WriteFile(os.ExpandEnv("$HOME/.local/share/remventory-dev/local.json"), b, 0600); err != nil {
		panic(err)
	}
}
