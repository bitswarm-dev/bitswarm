package health

import (
	"context"
	"testing"

	"bitswarm/server/internal/db"
)

func TestSQLRepository_RecordAndLatest(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	repo := NewRepository(conn)
	ctx := context.Background()

	if _, err := repo.Latest(ctx); err == nil {
		t.Error("Latest() on empty table expected error, got nil")
	}

	recorded, err := repo.Record(ctx)
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	latest, err := repo.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if latest.ID != recorded.ID {
		t.Errorf("Latest().ID = %d, want %d", latest.ID, recorded.ID)
	}
}
