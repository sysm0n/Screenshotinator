// Package export writes a curated session as a Markdown bundle (document + copied screenshots),
// ready for Obsidian or GitHub.
package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sysm0n/Screenshotinator/internal/models"
)

// ErrEmpty is returned when there is nothing left to export.
var ErrEmpty = errors.New("nothing to export: no screenshots left in the session")

// Lookup resolves screenshots by ID (implemented by *storage.DB).
type Lookup interface {
	Get(ctx context.Context, id int64) (*models.Screenshot, error)
}

// Step is one rendered section of the document.
type Step struct {
	Number    int
	Title     string
	Timestamp string
	Image     string // path relative to the document
	Notes     string
	Summary   string
}

// Write creates <folder>/<name>/<name>.md and <folder>/<name>/images/step-NN.jpg and returns the
// document path. Image paths always come from the store, never from the UI. On failure the
// partially written bundle is removed.
func Write(ctx context.Context, store Lookup, sessionID string, items []models.FrontendScreenshotData, folder string, now time.Time) (_ string, err error) {
	if len(items) == 0 {
		return "", ErrEmpty
	}
	if st, err := os.Stat(folder); err != nil || !st.IsDir() {
		return "", fmt.Errorf("export folder %q is not a directory", folder)
	}

	name := fmt.Sprintf("recall-%s-%s", now.Format("2006-01-02_150405"), shortID(sessionID))
	root := filepath.Join(folder, name)
	if err := os.Mkdir(root, 0o755); err != nil {
		return "", fmt.Errorf("create bundle: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(root)
		}
	}()
	if err := os.Mkdir(filepath.Join(root, "images"), 0o755); err != nil {
		return "", fmt.Errorf("create bundle: %w", err)
	}

	steps := make([]Step, 0, len(items))
	for i, it := range items {
		shot, err := store.Get(ctx, it.ID)
		if err != nil {
			return "", fmt.Errorf("step %d: %w", i+1, err)
		}
		if shot.SessionID != sessionID {
			return "", fmt.Errorf("step %d: screenshot %d belongs to another session", i+1, it.ID)
		}
		image := fmt.Sprintf("images/step-%02d.jpg", i+1)
		if err := copyFile(shot.ImagePath, filepath.Join(root, filepath.FromSlash(image))); err != nil {
			return "", fmt.Errorf("step %d: %w", i+1, err)
		}
		steps = append(steps, Step{
			Number:    i + 1,
			Title:     shot.AppName,
			Timestamp: shot.Timestamp,
			Image:     image,
			Notes:     it.UserNotes,
			Summary:   it.AISummary,
		})
	}

	doc := filepath.Join(root, name+".md")
	if err := os.WriteFile(doc, []byte(Render(steps)), 0o644); err != nil {
		return "", fmt.Errorf("write document: %w", err)
	}
	return doc, nil
}

// Render produces the Markdown document body.
func Render(steps []Step) string {
	var b strings.Builder
	for _, s := range steps {
		title := oneLine(s.Title)
		if title == "" {
			title = "Unknown window"
		}
		fmt.Fprintf(&b, "## Step %d – %s\n\n", s.Number, strings.ReplaceAll(title, "#", `\#`))
		fmt.Fprintf(&b, "*%s*\n\n", s.Timestamp)
		fmt.Fprintf(&b, "![Step %d](%s)\n\n", s.Number, s.Image)
		if n := strings.TrimSpace(s.Notes); n != "" {
			fmt.Fprintf(&b, "### Notes\n\n%s\n\n", n)
		}
		if sum := strings.TrimSpace(s.Summary); sum != "" {
			fmt.Fprintf(&b, "### AI Summary\n\n%s\n\n", sum)
		}
		b.WriteString("---\n\n")
	}
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
