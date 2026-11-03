package health

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"bitswarm/server/internal/logger"
)

type fakeRepository struct {
	recordResult Check
	recordErr    error
	latestResult Check
	latestErr    error
}

func (f *fakeRepository) Record(ctx context.Context) (Check, error) {
	return f.recordResult, f.recordErr
}

func (f *fakeRepository) Latest(ctx context.Context) (Check, error) {
	return f.latestResult, f.latestErr
}

func testLogger() *logger.Logger {
	return logger.New(logger.Options{Level: logger.LevelError, Output: &bytes.Buffer{}})
}

func TestService_Check(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{name: "success"},
		{name: "repository failure", repoErr: errors.New("db down"), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := Check{ID: 1, CheckedAt: time.Now()}
			repo := &fakeRepository{recordResult: want, recordErr: tc.repoErr}
			svc := NewService(repo, testLogger())

			got, err := svc.Check(context.Background())

			if (err != nil) != tc.wantErr {
				t.Fatalf("Check() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got.ID != want.ID {
				t.Errorf("Check().ID = %d, want %d", got.ID, want.ID)
			}
		})
	}
}

func TestService_Latest(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{name: "success"},
		{name: "no rows", repoErr: errors.New("no rows"), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := Check{ID: 2, CheckedAt: time.Now()}
			repo := &fakeRepository{latestResult: want, latestErr: tc.repoErr}
			svc := NewService(repo, testLogger())

			got, err := svc.Latest(context.Background())

			if (err != nil) != tc.wantErr {
				t.Fatalf("Latest() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got.ID != want.ID {
				t.Errorf("Latest().ID = %d, want %d", got.ID, want.ID)
			}
		})
	}
}
