package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sysm0n/Screenshotinator/internal/models"
)

type mapLookup map[int64]*models.Screenshot

func (m mapLookup) Get(_ context.Context, id int64) (*models.Screenshot, error) {
	if s, ok := m[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("id %d not found", id)
}

var now = time.Date(2026, 10, 20, 20, 38, 5, 0, time.UTC)

func fixture(t *testing.T) (mapLookup, string) {
	t.Helper()
	src := t.TempDir()
	shots := mapLookup{}
	for i, title := range []string{"Code – main.go", "Terminal #2"} {
		p := filepath.Join(src, fmt.Sprintf("%d.jpg", i+1))
		os.WriteFile(p, []byte(fmt.Sprintf("jpeg-%d", i+1)), 0o600)
		shots[int64(i+1)] = &models.Screenshot{ID: int64(i + 1), SessionID: "abcdef123456", Timestamp: "2026-10-20T20:38:00Z", AppName: title, ImagePath: p}
	}
	return shots, t.TempDir()
}

func TestWriteBundle(t *testing.T) {
	shots, out := fixture(t)
	items := []models.FrontendScreenshotData{
		{ID: 2, UserNotes: "ran the tests", AISummary: "go test output"}, // user reordered: 2 first
		{ID: 1},
	}
	doc, err := Write(context.Background(), shots, "abcdef123456", items, out, now)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(out, "recall-2026-10-20_203805-abcdef12", "recall-2026-10-20_203805-abcdef12.md"); doc != want {
		t.Errorf("doc = %s, want %s", doc, want)
	}
	md, _ := os.ReadFile(doc)
	text := string(md)

	for _, want := range []string{
		"## Step 1 – Terminal \\#2",
		"![Step 1](images/step-01.jpg)",
		"### Notes\n\nran the tests",
		"### AI Summary\n\ngo test output",
		"## Step 2 – Code – main.go",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("document missing %q:\n%s", want, text)
		}
	}
	if strings.HasPrefix(text, "# ") {
		t.Error("document must start with the first step, not a title header")
	}
	if strings.Count(text, "### Notes") != 1 || strings.Count(text, "### AI Summary") != 1 {
		t.Error("empty notes/summary sections should be omitted")
	}
	img, _ := os.ReadFile(filepath.Join(filepath.Dir(doc), "images", "step-01.jpg"))
	if string(img) != "jpeg-2" {
		t.Errorf("step-01.jpg = %q, want the second screenshot", img)
	}
}

func TestWriteRejectsForeignSessionAndCleansUp(t *testing.T) {
	shots, out := fixture(t)
	shots[2].SessionID = "someone-else"
	_, err := Write(context.Background(), shots, "abcdef123456",
		[]models.FrontendScreenshotData{{ID: 1}, {ID: 2}}, out, now)
	if err == nil || !strings.Contains(err.Error(), "another session") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("partial bundle left behind: %v", entries)
	}
}

func TestWriteEmpty(t *testing.T) {
	if _, err := Write(context.Background(), mapLookup{}, "s", nil, t.TempDir(), now); !errors.Is(err, ErrEmpty) {
		t.Errorf("err = %v, want ErrEmpty", err)
	}
}
