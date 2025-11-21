// internal/storage/marktypes/store.go
package marktypes

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

type MarkType struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	URL         *string `json:"url,omitempty"`
	Description *string `json:"description,omitempty"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, mt *MarkType) error {
	const q = `
		INSERT INTO mark_type (title, slug, url, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	return s.db.QueryRowContext(ctx, q,
		mt.Title,
		mt.Slug,
		mt.URL,
		mt.Description,
	).Scan(&mt.ID)
}

func (s *Store) List(ctx context.Context) ([]MarkType, error) {
	const q = `
		SELECT id, title, slug, url, description
		FROM mark_type
		ORDER BY id ASC;
	`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MarkType
	for rows.Next() {
		var mt MarkType
		var url, desc sql.NullString

		if err := rows.Scan(
			&mt.ID,
			&mt.Title,
			&mt.Slug,
			&url,
			&desc,
		); err != nil {
			return nil, err
		}

		if url.Valid {
			u := url.String
			mt.URL = &u
		}
		if desc.Valid {
			d := desc.String
			mt.Description = &d
		}

		out = append(out, mt)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
