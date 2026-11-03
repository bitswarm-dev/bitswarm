package docstore

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"bitswarm/server/internal/jsonutil"
)

var ErrNotFound = errors.New("not found")

type Collection[T any] struct {
	db   *sql.DB
	name string
	id   func(*T) *int64
	mu   sync.Mutex
}

func NewCollection[T any](db *sql.DB, name string, id func(*T) *int64) *Collection[T] {
	return &Collection[T]{db: db, name: name, id: id}
}

func decode[T any](body string) (T, error) {
	var v T
	err := jsonutil.UnmarshalStored([]byte(body), &v)
	return v, err
}

func (c *Collection[T]) List(ctx context.Context) ([]T, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT body FROM docs WHERE collection = ? ORDER BY id`, c.name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []T{}
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		v, err := decode[T](body)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (c *Collection[T]) Get(ctx context.Context, id int64) (T, error) {
	var body string
	err := c.db.QueryRowContext(ctx, `SELECT body FROM docs WHERE collection = ? AND id = ?`, c.name, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		var zero T
		return zero, ErrNotFound
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return decode[T](body)
}

func (c *Collection[T]) Create(ctx context.Context, v T) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var next int64
	err := c.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) + 1 FROM docs WHERE collection = ?`, c.name).Scan(&next)
	if err != nil {
		return v, err
	}
	*c.id(&v) = next
	body, err := jsonutil.Marshal(v)
	if err != nil {
		return v, err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO docs (collection, id, body) VALUES (?, ?, ?)`, c.name, next, string(body))
	return v, err
}

func (c *Collection[T]) Put(ctx context.Context, v T) error {
	body, err := jsonutil.Marshal(v)
	if err != nil {
		return err
	}
	res, err := c.db.ExecContext(ctx, `UPDATE docs SET body = ? WHERE collection = ? AND id = ?`, string(body), c.name, *c.id(&v))
	return affected(res, err)
}

func (c *Collection[T]) PutAll(ctx context.Context, items []T) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, v := range items {
		body, err := jsonutil.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE docs SET body = ? WHERE collection = ? AND id = ?`, string(body), c.name, *c.id(&v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Collection[T]) Delete(ctx context.Context, id int64) error {
	return affected(c.db.ExecContext(ctx, `DELETE FROM docs WHERE collection = ? AND id = ?`, c.name, id))
}

func (c *Collection[T]) DeleteWhere(ctx context.Context, drop func(T) bool) (int, error) {
	items, err := c.List(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, v := range items {
		if drop(v) {
			if err := c.Delete(ctx, *c.id(&v)); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (c *Collection[T]) Clear(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM docs WHERE collection = ?`, c.name)
	return err
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type KV struct {
	db *sql.DB
}

func NewKV(db *sql.DB) *KV {
	return &KV{db: db}
}

func (k *KV) Load(ctx context.Context, key string, dst any) (bool, error) {
	var body string
	err := k.db.QueryRowContext(ctx, `SELECT body FROM kv WHERE key = ?`, key).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, jsonutil.UnmarshalStored([]byte(body), dst)
}

func (k *KV) Save(ctx context.Context, key string, v any) error {
	body, err := jsonutil.Marshal(v)
	if err != nil {
		return err
	}
	_, err = k.db.ExecContext(ctx, `INSERT INTO kv (key, body) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET body = excluded.body`, key, string(body))
	return err
}

func (k *KV) Delete(ctx context.Context, key string) error {
	_, err := k.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
	return err
}
