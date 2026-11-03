package torrents

import (
	"context"
	"database/sql"

	"bitswarm/server/internal/docstore"
)

type Repository interface {
	List(ctx context.Context) ([]Torrent, error)
	Get(ctx context.Context, id int64) (Torrent, error)
	Create(ctx context.Context, t Torrent) (Torrent, error)
	Put(ctx context.Context, t Torrent) error
	PutAll(ctx context.Context, items []Torrent) error
	Delete(ctx context.Context, id int64) error
}

func NewRepository(db *sql.DB) Repository {
	return docstore.NewCollection(db, "torrents", func(t *Torrent) *int64 { return &t.ID })
}
