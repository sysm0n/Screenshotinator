package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sysm0n/Screenshotinator/internal/models"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestScreenshotLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)

	a := &models.Screenshot{SessionID: "s1", Timestamp: "2026-01-01T12:00:00Z", AppName: "Code", ImagePath: "/tmp/a.jpg"}
	b := &models.Screenshot{SessionID: "s1", Timestamp: "2026-01-01T12:00:02Z", AppName: "Terminal", ImagePath: "/tmp/b.jpg"}
	other := &models.Screenshot{SessionID: "s2", Timestamp: "2026-01-01T12:00:03Z", ImagePath: "/tmp/c.jpg"}
	for _, s := range []*models.Screenshot{a, b, other} {
		if err := db.Insert(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if a.ID == 0 || b.ID <= a.ID {
		t.Fatalf("IDs not assigned in order: %d, %d", a.ID, b.ID)
	}

	if err := db.SetNotes(ctx, a.ID, "opened project"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSummary(ctx, a.ID, "VS Code with main.go"); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserNotes != "opened project" || got.AISummary != "VS Code with main.go" || got.AppName != "Code" {
		t.Errorf("Get = %+v", got)
	}

	list, err := db.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("ListBySession = %+v", list)
	}

	path, err := db.Delete(ctx, a.ID)
	if err != nil || path != "/tmp/a.jpg" {
		t.Fatalf("Delete = %q, %v", path, err)
	}
	if _, err := db.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want ErrNotFound", err)
	}
}

func TestNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	if _, err := db.Get(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: %v", err)
	}
	if err := db.SetNotes(ctx, 42, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetNotes: %v", err)
	}
	if _, err := db.Delete(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete: %v", err)
	}
	list, err := db.ListBySession(ctx, "none")
	if err != nil || list == nil || len(list) != 0 {
		t.Errorf("ListBySession(empty) = %#v, %v; want non-nil empty slice", list, err)
	}
}
