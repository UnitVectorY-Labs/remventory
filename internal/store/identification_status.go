package store

import "context"

// IdentificationStatuses loads the visible page in one query, without external calls.
func (s *Store) IdentificationStatuses(ctx context.Context, items []Item) (map[string]string, error) {
	result := map[string]string{}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `select i.id::text, case
 when exists(select 1 from item_canonical_links l where l.item_id=i.id) then 'identified'
 when exists(select 1 from identification_jobs j where j.item_id=i.id and j.state='review') then 'review'
 when exists(select 1 from identification_jobs j where j.item_id=i.id and j.state in ('queued','processing')) then 'processing'
 else 'unidentified' end from items i where i.id::text=any($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		if err = rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		result[id] = status
	}
	return result, rows.Err()
}
