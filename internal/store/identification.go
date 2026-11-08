package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type IdentificationInput struct {
	DefaultTags   []string        `json:"default_tags,omitempty"`
	DefaultValues json.RawMessage `json:"default_values,omitempty"`
	FieldKeys     []string        `json:"field_keys,omitempty"`
	Policy        *IntakeConfig   `json:"policy,omitempty"`
	Text          string          `json:"text"`
	TemplateID    string          `json:"template_id,omitempty"`
}
type IdentificationMatch struct {
	Display    authority.DisplayPlan `json:"display"`
	Role       string                `json:"role,omitempty"`
	Candidate  authority.Candidate   `json:"candidate"`
	Graph      authority.Graph       `json:"graph"`
	Confidence string                `json:"confidence"`
	Reason     string                `json:"reason"`
}
type IdentificationResult struct {
	Outcome  string                `json:"outcome"`
	Title    string                `json:"title"`
	Notes    string                `json:"notes"`
	Inferred map[string]any        `json:"inferred"`
	Matches  []IdentificationMatch `json:"matches"`
	Searches []map[string]any      `json:"searches"`
	Warnings []string              `json:"warnings"`
}
type IdentificationPhoto struct {
	ID           string `json:"id"`
	JobID        string `json:"job_id"`
	Key          string `json:"-"`
	ThumbnailKey string `json:"-"`
	MIME         string `json:"mime_type"`
	Filename     string `json:"filename"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Size         int64  `json:"size"`
}
type IdentificationJob struct {
	ID         string                `json:"id"`
	ItemID     string                `json:"item_id"`
	Category   string                `json:"category"`
	Input      IdentificationInput   `json:"input"`
	State      string                `json:"state"`
	Attempts   int                   `json:"attempts"`
	Result     IdentificationResult  `json:"result"`
	Error      string                `json:"error"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	LeaseToken string                `json:"-"`
	Photos     []IdentificationPhoto `json:"photos"`
}

const jobColumns = `id,coalesce(item_id::text,''),category,input,state,attempts,result,error,created_at,updated_at,coalesce(lease_token::text,'')`

func scanJob(row rowScanner) (IdentificationJob, error) {
	var j IdentificationJob
	err := row.Scan(&j.ID, &j.ItemID, &j.Category, &j.Input, &j.State, &j.Attempts, &j.Result, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.LeaseToken)
	return j, err
}
func (s *Store) CreateIdentification(ctx context.Context, category, itemID string, input IdentificationInput, photos []IdentificationPhoto) (IdentificationJob, error) {
	if input.Policy == nil {
		var profile Category
		var e error
		if input.TemplateID != "" {
			profile, e = s.GetCategoryDefinition(ctx, input.TemplateID)
		} else if itemID != "" {
			var item Item
			item, e = s.GetItem(ctx, itemID)
			if e == nil {
				profile, e = s.ProfileForItem(ctx, item)
			}
		} else {
			var profiles []Category
			profiles, e = s.ListCategories(ctx, 200, 0)
			for _, p := range profiles {
				if strings.EqualFold(p.Name, category) {
					profile = p
					break
				}
			}
		}
		if e != nil {
			return IdentificationJob{}, e
		}
		if profile.ID != "" {
			input.TemplateID = profile.ID
			input.DefaultTags = profile.DefaultTags
			input.DefaultValues = profile.DefaultValues
			for _, field := range profile.Attributes {
				input.FieldKeys = append(input.FieldKeys, field.Key)
			}
			policy := profile.IntakeConfig
			input.Policy = &policy
			category = profile.Name
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IdentificationJob{}, err
	}
	defer tx.Rollback(ctx)
	if itemID != "" {
		var owner string
		if err = tx.QueryRow(ctx, `select user_id from items where id=$1`, itemID).Scan(&owner); err != nil {
			return IdentificationJob{}, err
		}
		if owner != DefaultUserID {
			return IdentificationJob{}, ErrNotFound
		}
	}
	b, _ := json.Marshal(input)
	j, err := scanJob(tx.QueryRow(ctx, `insert into identification_jobs(user_id,item_id,category,input) values($1,nullif($2,'')::uuid,$3,$4) returning `+jobColumns, DefaultUserID, itemID, category, b))
	if err != nil {
		return j, err
	}
	for _, p := range photos {
		_, err = tx.Exec(ctx, `insert into identification_photos(id,job_id,object_key,thumbnail_key,mime_type,filename,width,height,size_bytes) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, j.ID, p.Key, p.ThumbnailKey, p.MIME, p.Filename, p.Width, p.Height, p.Size)
		if err != nil {
			return j, err
		}
	}
	return j, tx.Commit(ctx)
}
func (s *Store) Identification(ctx context.Context, id string) (IdentificationJob, error) {
	j, err := scanJob(s.pool.QueryRow(ctx, `select `+jobColumns+` from identification_jobs where id=$1 and user_id=$2`, id, DefaultUserID))
	if err != nil {
		return j, err
	}
	j.Photos, err = s.IdentificationPhotos(ctx, id)
	return j, err
}
func (s *Store) IdentificationPhotos(ctx context.Context, id string) ([]IdentificationPhoto, error) {
	rows, err := s.pool.Query(ctx, `select id,job_id,object_key,thumbnail_key,mime_type,filename,width,height,size_bytes from identification_photos where job_id=$1
 union all select a.id,j.id,a.object_key,coalesce(a.thumbnail_object_key,a.object_key),a.mime_type,a.original_filename,a.width,a.height,a.size_bytes from identification_jobs j join item_assets a on a.item_id=j.item_id where j.id=$1 and not exists(select 1 from identification_photos where job_id=$1) order by id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IdentificationPhoto{}
	for rows.Next() {
		var p IdentificationPhoto
		if err = rows.Scan(&p.ID, &p.JobID, &p.Key, &p.ThumbnailKey, &p.MIME, &p.Filename, &p.Width, &p.Height, &p.Size); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) Identifications(ctx context.Context) ([]IdentificationJob, error) {
	rows, err := s.pool.Query(ctx, `select `+jobColumns+` from identification_jobs where user_id=$1 order by created_at desc limit 200`, DefaultUserID)
	if err != nil {
		return nil, err
	}
	out := []IdentificationJob{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, j)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Photos, err = s.IdentificationPhotos(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) ClaimIdentification(ctx context.Context) (IdentificationJob, error) {
	return scanJob(s.pool.QueryRow(ctx, `update identification_jobs set state='processing',attempts=attempts+1,lease_token=gen_random_uuid(),lease_until=now()+interval '12 minutes',updated_at=now() where id=(select id from identification_jobs where (state='queued' and available_at<=now()) or (state='processing' and lease_until<now()) order by created_at for update skip locked limit 1) returning `+jobColumns))
}
func (s *Store) FinishIdentification(ctx context.Context, j IdentificationJob, result IdentificationResult, processErr error) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	state, msg := "review", ""
	if processErr != nil {
		msg = processErr.Error()
		state = "failed"
		if j.Attempts < 3 {
			state = "queued"
		}
	}
	tag, err := s.pool.Exec(ctx, `update identification_jobs set state=$3,result=$4,error=$5,lease_until=null,available_at=now()+interval '45 seconds',updated_at=now() where id=$1 and lease_token=$2 and state='processing'`, j.ID, j.LeaseToken, state, b, msg)
	if err == nil && tag.RowsAffected() == 0 {
		return fmt.Errorf("job lease lost")
	}
	return err
}
func (s *Store) SetIdentificationState(ctx context.Context, id, action string) error {
	var tag int64
	switch action {
	case "retry":
		r, e := s.pool.Exec(ctx, `update identification_jobs set state='queued',attempts=0,error='',available_at=now(),updated_at=now() where id=$1 and state in ('failed','review','search_only','rejected')`, id)
		tag = r.RowsAffected()
		if e != nil {
			return e
		}
	case "reject", "search_only":
		state := "rejected"
		if action == "search_only" {
			state = action
		}
		r, e := s.pool.Exec(ctx, `update identification_jobs set state=$2,updated_at=now() where id=$1 and state in ('review','failed','queued')`, id, state)
		tag = r.RowsAffected()
		if e != nil {
			return e
		}
	default:
		return fmt.Errorf("unsupported action")
	}
	if tag == 0 {
		return fmt.Errorf("job is not available for this action")
	}
	return nil
}
func (s *Store) ReplaceIdentificationResult(ctx context.Context, id string, result IdentificationResult) error {
	b, _ := json.Marshal(result)
	tag, err := s.pool.Exec(ctx, `update identification_jobs set result=$2,updated_at=now() where id=$1 and state='review'`, id, b)
	if err == nil && tag.RowsAffected() == 0 {
		return fmt.Errorf("job is no longer in review")
	}
	return err
}

type IdentificationProposal struct {
	ReplaceExisting       bool            `json:"replace_existing,omitempty"`
	ExistingLinksSnapshot string          `json:"existing_links_snapshot,omitempty"`
	ConfirmAlternative    bool            `json:"confirm_alternative,omitempty"`
	JobID                 string          `json:"job_id"`
	Selected              []int           `json:"selected"`
	Title                 string          `json:"title"`
	Tags                  []string        `json:"tags"`
	Attributes            json.RawMessage `json:"attributes"`
	ItemID                string          `json:"item_id,omitempty"`
	ResultSnapshot        string          `json:"result_snapshot"`
}

func (s *Store) ProposeIdentification(ctx context.Context, p IdentificationProposal) (Proposal, error) {
	j, err := s.Identification(ctx, p.JobID)
	if err != nil {
		return Proposal{}, err
	}
	if j.State != "review" {
		return Proposal{}, fmt.Errorf("job must be in review")
	}
	if err = validSelection(j, p.Selected, p.ConfirmAlternative); err != nil {
		return Proposal{}, err
	}
	p.ItemID = j.ItemID
	if p.ReplaceExisting {
		if j.ItemID == "" || len(p.Selected) == 0 {
			return Proposal{}, fmt.Errorf("select a replacement for an existing item")
		}
		if err = s.pool.QueryRow(ctx, `select coalesce(jsonb_agg(jsonb_build_array(entity_id,relation) order by entity_id,relation),'[]'::jsonb)::text from item_canonical_links where item_id=$1`, j.ItemID).Scan(&p.ExistingLinksSnapshot); err != nil {
			return Proposal{}, err
		}
	}
	snapshot, _ := json.Marshal(j.Result)
	p.ResultSnapshot = fmt.Sprintf("%x", sha256.Sum256(snapshot))
	if p.ItemID == "" {
		if len(p.Attributes) == 0 {
			p.Attributes = []byte(`{}`)
		}
		templateID := j.Input.TemplateID
		if len(j.Input.DefaultValues) > 0 {
			values := map[string]any{}
			_ = json.Unmarshal(j.Input.DefaultValues, &values)
			var entered map[string]any
			if err := json.Unmarshal(p.Attributes, &entered); err != nil {
				return Proposal{}, err
			}
			for k, v := range entered {
				values[k] = v
			}
			p.Attributes, _ = json.Marshal(values)
			p.Tags = append(append([]string{}, j.Input.DefaultTags...), p.Tags...)
			templateID = ""
		}
		prepared, err := s.prepareItem(ctx, ItemProposalPayload{Operation: "create", CategoryID: templateID, Title: p.Title, Tags: p.Tags, Attributes: p.Attributes})
		if err != nil {
			return Proposal{}, err
		}
		if err := validateIdentificationConsole(j, prepared.Attributes); err != nil {
			return Proposal{}, err
		}
		p.Attributes = prepared.Attributes
		p.Tags = identificationTags(j, prepared.Tags, prepared.Attributes)
		p.Title = prepared.Title
	}
	return s.createProposal(ctx, DefaultUserID, "identification_link", p)
}
func validSelection(j IdentificationJob, selected []int, confirmAlternative ...bool) error {
	seen := map[int]bool{}
	for _, i := range selected {
		if i < 0 || i >= len(j.Result.Matches) || seen[i] {
			return fmt.Errorf("invalid candidate selection")
		}
		if j.Result.Matches[i].Confidence == "rejected" && !(len(confirmAlternative) > 0 && confirmAlternative[0]) {
			return fmt.Errorf("candidate was ruled out; search for another match")
		}
		seen[i] = true
	}
	return nil
}
func applyIdentification(ctx context.Context, tx pgx.Tx, proposal *Proposal) error {
	var p IdentificationProposal
	if err := json.Unmarshal(proposal.ProposedPayload, &p); err != nil {
		return err
	}
	j, err := scanJob(tx.QueryRow(ctx, `select `+jobColumns+` from identification_jobs where id=$1 for update`, p.JobID))
	if err != nil {
		return err
	}
	if j.State != "review" {
		return fmt.Errorf("job has already been decided or retried")
	}
	snapshot, _ := json.Marshal(j.Result)
	if fmt.Sprintf("%x", sha256.Sum256(snapshot)) != p.ResultSnapshot {
		return fmt.Errorf("identification changed; create a new proposal")
	}
	if err = validSelection(j, p.Selected, p.ConfirmAlternative); err != nil {
		return err
	}
	itemID := j.ItemID
	if itemID == "" {
		fields, err := readFields(ctx, tx)
		if err != nil {
			return err
		}
		if err = validateIdentificationConsole(j, p.Attributes); err != nil {
			return err
		}
		if err = validateItemAttributeValues(p.Attributes, fieldAttributes(fields)); err != nil {
			return err
		}
		itemID, err = applyItemChange(ctx, tx, proposal.UserID, ItemProposalPayload{Operation: "create", Title: p.Title, Attributes: p.Attributes, Tags: p.Tags, Quantity: 1})
		if err != nil {
			return err
		}
		// Photo attachment and primary choice are part of the same explicit approval.
		_, err = tx.Exec(ctx, `insert into item_assets(id,item_id,object_key,thumbnail_object_key,mime_type,original_filename,width,height,size_bytes,is_primary) select id,$2,object_key,thumbnail_key,mime_type,filename,width,height,size_bytes,id=(select id from identification_photos where job_id=$1 order by id limit 1) from identification_photos where job_id=$1`, j.ID, itemID)
		if err != nil {
			return err
		}
	}
	if j.ItemID != "" {
		// Serialize confirmations for one owned item, including replacement proposals.
		var locked string
		if err = tx.QueryRow(ctx, `select id::text from items where id=$1 for update`, itemID).Scan(&locked); err != nil {
			return err
		}
	}
	if p.ReplaceExisting {
		if j.ItemID == "" || len(p.Selected) == 0 {
			return fmt.Errorf("select a replacement for an existing item")
		}
		var current string
		if err = tx.QueryRow(ctx, `select coalesce(jsonb_agg(jsonb_build_array(entity_id,relation) order by entity_id,relation),'[]'::jsonb)::text from item_canonical_links where item_id=$1`, itemID).Scan(&current); err != nil {
			return err
		}
		if current != p.ExistingLinksSnapshot {
			return fmt.Errorf("linked matches changed; review a new proposal")
		}
		if _, err = tx.Exec(ctx, `delete from item_canonical_links where item_id=$1`, itemID); err != nil {
			return err
		}
	}
	for _, i := range p.Selected {
		match := j.Result.Matches[i]
		ref := match.Graph.Root
		relation := match.Graph.Association
		if relation != "identified_as" && relation != "contains" {
			relation = "associated_with"
		}
		if match.Role == "component" {
			relation = "contains"
		}
		confidence := match.Confidence
		if p.ConfirmAlternative && confidence == "rejected" {
			confidence = "user_confirmed"
		}
		_, err = tx.Exec(ctx, `insert into item_canonical_links(item_id,entity_id,relation,job_id,proposal_id,confidence) select $1,entity_id,$5,$6,$7,$8 from canonical_identifiers where provider=$2 and kind=$3 and external_id=$4 on conflict do nothing`, itemID, ref.Provider, ref.Kind, ref.ID, relation, j.ID, proposal.ID, confidence)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `update identification_jobs set state='approved',item_id=$2,updated_at=now() where id=$1`, j.ID, itemID)
	return err
}

// Batch enrichment uses the exact same durable worker and review path as photos.
func (s *Store) QueueEnrichment(ctx context.Context, tag string, limit int) ([]IdentificationJob, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	items, _, err := s.SearchLibrary(ctx, "", "", "", "", tag, "", limit, 0)
	if err != nil {
		return nil, err
	}
	out := []IdentificationJob{}
	for _, i := range items {
		category := tag
		if category == "" {
			for _, t := range i.Tags {
				category += " " + t
			}
		}
		j, e := s.CreateIdentification(ctx, category, i.ID, IdentificationInput{Text: i.Title + "\nOriginal user-entered fields: " + string(i.Attributes)}, nil)
		if e != nil {
			var active bool
			_ = s.pool.QueryRow(ctx, `select exists(select 1 from identification_jobs where item_id=$1 and state in ('queued','processing','review'))`, i.ID).Scan(&active)
			if active {
				continue
			}
			return out, e
		}
		out = append(out, j)
	}
	return out, nil
}

// Templates with a Console field require an owned-copy console during intake.
// The selected catalogue work may list many platforms or omit platform claims.
func validateIdentificationConsole(j IdentificationJob, attributes json.RawMessage) error {
	if j.ItemID != "" {
		return nil
	}
	for _, key := range j.Input.FieldKeys {
		if key != "console" {
			continue
		}
		var values map[string]any
		if err := json.Unmarshal(attributes, &values); err != nil {
			return err
		}
		console, _ := values[key].(string)
		if strings.TrimSpace(console) == "" {
			return fmt.Errorf("confirm the console on the game packaging before saving")
		}
	}
	return nil
}

// Old intake jobs retain their original template tags. Correct those defaults
// when the owner confirms a console, without changing the pending job itself.
func identificationTags(j IdentificationJob, tags []string, attributes json.RawMessage) []string {
	hasConsole := false
	for _, key := range j.Input.FieldKeys {
		if key == "console" {
			hasConsole = true
		}
	}
	var values map[string]any
	_ = json.Unmarshal(attributes, &values)
	console, _ := values["console"].(string)
	console = strings.TrimSpace(console)
	result := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || (hasConsole && console != "" && strings.EqualFold(tag, "Consoles")) {
			continue
		}
		key := strings.ToLower(tag)
		if !seen[key] {
			result = append(result, tag)
			seen[key] = true
		}
	}
	if hasConsole && console != "" && !seen[strings.ToLower(console)] {
		result = append(result, console)
	}
	return result
}
