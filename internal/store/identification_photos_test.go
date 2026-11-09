package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestAddIdentificationPhotosRetriesSameDraft(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	photo := func() IdentificationPhoto {
		return IdentificationPhoto{ID: uuid.NewString(), Key: "original", ThumbnailKey: "thumb", MIME: "image/jpeg", Filename: "photo.jpg", Width: 10, Height: 10, Size: 100}
	}
	original, added := photo(), photo()
	job, err := s.CreateIdentification(ctx, "Object", "", IdentificationInput{Text: "original clue"}, []IdentificationPhoto{original})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimIdentification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddIdentificationPhotos(ctx, job.ID, []IdentificationPhoto{added}); err == nil {
		t.Fatal("changed a processing job")
	}
	if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Title: "old guess"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddIdentificationPhotos(ctx, job.ID, []IdentificationPhoto{added}); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Identification(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != "queued" || updated.Result.Title != "" || updated.Input.Text != "original clue" || len(updated.Photos) != 2 {
		t.Fatalf("draft was not reset with both photos: %+v", updated)
	}
	if _, err = s.AddIdentificationPhotos(ctx, job.ID, []IdentificationPhoto{photo()}); err == nil {
		t.Fatal("accepted duplicate queued submission")
	}
	if _, err = s.pool.Exec(ctx, `update identification_jobs set state='failed' where id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	tooMany := []IdentificationPhoto{}
	for i := 0; i < 7; i++ {
		tooMany = append(tooMany, photo())
	}
	if _, err = s.AddIdentificationPhotos(ctx, job.ID, tooMany); err == nil {
		t.Fatal("exceeded eight photos")
	}
	updated, err = s.Identification(ctx, job.ID)
	if err != nil || len(updated.Photos) != 2 || updated.State != "failed" {
		t.Fatal("rejected upload changed draft", err)
	}
	if _, err = s.AddIdentificationPhotos(ctx, job.ID, []IdentificationPhoto{photo()}); err != nil {
		t.Fatal("failed draft could not retry", err)
	}
}
