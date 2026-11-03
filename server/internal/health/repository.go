package health

import (
	"context"
	"database/sql"
)

type Repository interface {
	Record(ctx context.Context) (Check, error)
	Latest(ctx context.Context) (Check, error)
}

type sqlRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

func (r *sqlRepository) Record(ctx context.Context) (Check, error) {
	var c Check
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO health_checks DEFAULT VALUES RETURNING id, checked_at`,
	).Scan(&c.ID, &c.CheckedAt)
	return c, err
}

func (r *sqlRepository) Latest(ctx context.Context) (Check, error) {
	var c Check
	err := r.db.QueryRowContext(ctx,
		`SELECT id, checked_at FROM health_checks ORDER BY id DESC LIMIT 1`,
	).Scan(&c.ID, &c.CheckedAt)
	return c, err
}
