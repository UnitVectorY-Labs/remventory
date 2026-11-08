package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPendingIdentificationsPaginationAndPhotos(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	// More than 200 newer completed jobs must never hide an older pending job.
	_, err := s.pool.Exec(ctx, `insert into identification_jobs(user_id,state,created_at,result)
select $1,'approved',now()+n*interval '1 second',jsonb_build_object('searches',repeat('large search result ',10000)) from generate_series(1,205) n`, DefaultUserID)
	if err != nil {
		t.Fatal(err)
	}
	var firstID, photoID, itemID, assetID string
	if err := s.pool.QueryRow(ctx, `insert into items(user_id,title) values($1,'Owned copy') returning id`, DefaultUserID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `insert into item_assets(item_id,object_key,mime_type) values($1,'owned-original','image/png') returning id`, itemID).Scan(&assetID); err != nil {
		t.Fatal(err)
	}
	for i, state := range []string{"review", "failed", "queued", "processing", "review", "review", "review"} {
		var id string
		title, ownedItemID := "", ""
		if i == 0 {
			title = "Identified title"
		}
		if i == 6 {
			ownedItemID = itemID
		}
		if err := s.pool.QueryRow(ctx, `insert into identification_jobs(user_id,state,category,input,result,item_id,created_at)
values($1,$2,'Books','{"text":"Fallback title\nMore clues"}',jsonb_build_object('title',$3::text,'searches',repeat('must not be returned ',10000)),nullif($4,'')::uuid,now()-$5*interval '1 second') returning id`, DefaultUserID, state, title, ownedItemID, i).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstID = id
			if err := s.pool.QueryRow(ctx, `insert into identification_photos(id,job_id,object_key,thumbnail_key,mime_type,filename,width,height,size_bytes)
values(gen_random_uuid(),$1,'original','thumb','image/png','photo.png',1,1,10) returning id`, id).Scan(&photoID); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := s.PendingIdentifications(ctx, 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 7 || !page.Processing || len(page.Jobs) != 6 {
		t.Fatalf("wrong first page: %+v", page)
	}
	if page.Jobs[0].ID != firstID || page.Jobs[0].Title != "Identified title" || page.Jobs[0].PhotoID != photoID || page.Jobs[1].Title != "Fallback title" {
		t.Fatalf("wrong summaries: %+v", page.Jobs)
	}
	encoded, _ := json.Marshal(page)
	if len(encoded) > 3000 || strings.Contains(string(encoded), "must not be returned") {
		t.Fatalf("full results leaked into summaries: %d bytes", len(encoded))
	}
	last, err := s.PendingIdentifications(ctx, 6, 6)
	if err != nil || len(last.Jobs) != 1 || last.Jobs[0].PhotoID != assetID {
		t.Fatalf("missing fallback owned photo: %+v, %v", last, err)
	}
	for _, pair := range [][2]string{{firstID, photoID}, {last.Jobs[0].ID, assetID}} {
		photo, err := s.IdentificationPhoto(ctx, pair[0], pair[1])
		if err != nil || photo.ID != pair[1] {
			t.Fatalf("cannot load listed photo: %+v, %v", photo, err)
		}
	}
	if _, err := s.IdentificationPhoto(ctx, firstID, assetID); err == nil {
		t.Fatal("returned another job's photo")
	}
	var foreignJobID, foreignPhotoID string
	if err := s.pool.QueryRow(ctx, `with owner as (
 insert into users(display_name) values('Other owner') returning id
), job as (
 insert into identification_jobs(user_id,state) select id,'review' from owner returning id
)
insert into identification_photos(id,job_id,object_key,thumbnail_key,mime_type,filename,width,height,size_bytes)
select gen_random_uuid(),id,'private','private-thumb','image/png','private.png',1,1,1 from job returning job_id,id`).Scan(&foreignJobID, &foreignPhotoID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IdentificationPhoto(ctx, foreignJobID, foreignPhotoID); err == nil {
		t.Fatal("returned another owner's photo")
	}
	empty, err := s.PendingIdentifications(ctx, 6, 999)
	if err != nil || empty.Total != 7 || empty.Jobs == nil || len(empty.Jobs) != 0 {
		t.Fatalf("bad empty page: %+v, %v", empty, err)
	}
	if _, err := s.pool.Exec(ctx, `update identification_jobs set state='approved'`); err != nil {
		t.Fatal(err)
	}
	empty, err = s.PendingIdentifications(ctx, 6, 0)
	if err != nil || empty.Total != 0 || empty.Processing || len(empty.Jobs) != 0 {
		t.Fatalf("bad completed list: %+v, %v", empty, err)
	}
}
