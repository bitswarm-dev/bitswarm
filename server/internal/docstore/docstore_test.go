package docstore

import (
	"context"
	"errors"
	"testing"

	"bitswarm/server/internal/db"
)

type item struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func newItems(t *testing.T) (*Collection[item], *KV) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewCollection(conn, "items", func(i *item) *int64 { return &i.ID }), NewKV(conn)
}

func TestCollection(t *testing.T) {
	ctx := context.Background()
	col, _ := newItems(t)

	a, _ := col.Create(ctx, item{Name: "a"})
	b, _ := col.Create(ctx, item{Name: "b"})
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids = %d, %d", a.ID, b.ID)
	}

	b.Name = "b2"
	if err := col.PutAll(ctx, []item{b}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{"get existing", func() error { _, err := col.Get(ctx, 1); return err }, nil},
		{"get missing", func() error { _, err := col.Get(ctx, 9); return err }, ErrNotFound},
		{"put missing", func() error { return col.Put(ctx, item{ID: 9}) }, ErrNotFound},
		{"delete existing", func() error { return col.Delete(ctx, 1) }, nil},
		{"delete again", func() error { return col.Delete(ctx, 1) }, ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	items, _ := col.List(ctx)
	if len(items) != 1 || items[0].Name != "b2" {
		t.Errorf("List() = %+v", items)
	}

	n, _ := col.DeleteWhere(ctx, func(i item) bool { return i.Name == "b2" })
	if n != 1 {
		t.Errorf("DeleteWhere() = %d, want 1", n)
	}
	c, _ := col.Create(ctx, item{Name: "c"})
	if c.ID != 1 {
		t.Errorf("id after clearing = %d, want 1", c.ID)
	}
	if err := col.Clear(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestKV(t *testing.T) {
	ctx := context.Background()
	_, kv := newItems(t)

	var got item
	if ok, err := kv.Load(ctx, "x", &got); ok || err != nil {
		t.Fatalf("Load() missing = %v, %v", ok, err)
	}
	if err := kv.Save(ctx, "x", item{ID: 4, Name: "n"}); err != nil {
		t.Fatal(err)
	}
	if err := kv.Save(ctx, "x", item{ID: 5, Name: "m"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := kv.Load(ctx, "x", &got); !ok || err != nil || got.ID != 5 {
		t.Fatalf("Load() = %v, %v, %+v", ok, err, got)
	}
	if err := kv.Delete(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := kv.Load(ctx, "x", &got); ok {
		t.Error("Load() after delete = true")
	}
}
