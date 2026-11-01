package store

import "context"

type Facet struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type Library struct {
	Total int     `json:"total"`
	Tags  []Facet `json:"tags"`
}

func (s *Store) LibraryFacets(ctx context.Context) (Library, error) {
	result := Library{Tags: []Facet{}}
	if err := s.pool.QueryRow(ctx, `select count(*) from items where user_id=$1`, DefaultUserID).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := s.pool.Query(ctx, `select tag,count(*) from items cross join lateral unnest(tags) as tag where user_id=$1 group by tag order by lower(tag)`, DefaultUserID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var facet Facet
		if err := rows.Scan(&facet.Name, &facet.Count); err != nil {
			return result, err
		}
		result.Tags = append(result.Tags, facet)
	}
	return result, rows.Err()
}
