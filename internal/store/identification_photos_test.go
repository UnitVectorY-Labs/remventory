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

func TestIdentificationPrimaryPhotoApproval(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	for _, explicit := range []bool{false, true} {
		photos := []IdentificationPhoto{}
		for i := 0; i < 3; i++ {
			photos = append(photos, IdentificationPhoto{ID: uuid.NewString(), Key: "original", ThumbnailKey: "thumb", MIME: "image/jpeg", Filename: "photo.jpg", Width: 10, Height: 10, Size: 100})
		}
		job, err := s.CreateIdentification(ctx, "Object", "", IdentificationInput{}, photos)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := s.ClaimIdentification(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Title: "Object"}, nil); err != nil {
			t.Fatal(err)
		}
		job, err = s.Identification(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		draft := IdentificationProposal{JobID: job.ID, Title: "Object"}
		draft.PrimaryPhotoID = uuid.NewString()
		if _, err = s.ProposeIdentification(ctx, draft); err == nil {
			t.Fatal("accepted a photo outside this job")
		}
		draft.PrimaryPhotoID = ""
		want := job.Photos[0].ID
		if explicit {
			want = job.Photos[2].ID
			draft.PrimaryPhotoID = want
		}
		proposal, err := s.ProposeIdentification(ctx, draft)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err = s.pool.QueryRow(ctx, `select count(*) from item_assets where id=$1`, want).Scan(&count); err != nil || count != 0 {
			t.Fatal("proposal attached photo before approval", err)
		}
		if _, err = s.DecideProposal(ctx, proposal.ID, ProposalDecision{Approve: true}); err != nil {
			t.Fatal(err)
		}
		job, err = s.Identification(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		var primary string
		if err = s.pool.QueryRow(ctx, `select id::text from item_assets where item_id=$1 and is_primary`, job.ItemID).Scan(&primary); err != nil || primary != want {
			t.Fatalf("primary = %s, want %s: %v", primary, want, err)
		}
		if err = s.pool.QueryRow(ctx, `select count(*) from item_assets where item_id=$1`, job.ItemID).Scan(&count); err != nil || count != 3 {
			t.Fatal("all photos must be attached", err)
		}
		if explicit {
			review, err := s.CreateIdentification(ctx, "Object", job.ItemID, IdentificationInput{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			claimed, err := s.ClaimIdentification(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Title: "Object"}, nil); err != nil {
				t.Fatal(err)
			}
			proposal, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: review.ID, PrimaryPhotoID: job.Photos[0].ID})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.pool.QueryRow(ctx, `select id::text from item_assets where item_id=$1 and is_primary`, job.ItemID).Scan(&primary); err != nil || primary != want {
				t.Fatal("existing primary changed before approval", err)
			}
			if _, err = s.DecideProposal(ctx, proposal.ID, ProposalDecision{Approve: true}); err != nil {
				t.Fatal(err)
			}
			if err = s.pool.QueryRow(ctx, `select id::text from item_assets where item_id=$1 and is_primary`, job.ItemID).Scan(&primary); err != nil || primary != job.Photos[0].ID {
				t.Fatal("existing primary did not change on approval", err)
			}
		}
	}
}
