package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeSendsSingleImageRequest(t *testing.T) {
	var gotAuth string
	var got struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"  A terminal running go test.  "}}]}`)
	}))
	defer srv.Close()

	c := New(time.Second, func() Endpoint { return Endpoint{URL: srv.URL, Model: "llama3.2-vision"} })
	out, err := c.Analyze(context.Background(), Request{JPEG: []byte{1, 2, 3}, WindowTitle: "Terminal", Instructions: "name the failing test"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "A terminal running go test." {
		t.Errorf("summary = %q", out)
	}
	if gotAuth != "" {
		t.Errorf("Authorization sent with empty API key: %q", gotAuth)
	}
	if got.Model != "llama3.2-vision" || len(got.Messages) != 2 {
		t.Fatalf("request = %+v", got)
	}

	var parts []contentPart
	if err := json.Unmarshal(got.Messages[1].Content, &parts); err != nil {
		t.Fatal(err)
	}
	images := 0
	for _, p := range parts {
		if p.Type == "image_url" {
			images++
			if !strings.HasPrefix(p.ImageURL.URL, "data:image/jpeg;base64,") {
				t.Errorf("image URL = %.40s", p.ImageURL.URL)
			}
		}
		if p.Type == "text" && !strings.Contains(p.Text, "name the failing test") {
			t.Errorf("instructions missing from prompt: %q", p.Text)
		}
	}
	if images != 1 {
		t.Errorf("request carries %d images, want exactly 1 (DGX limit)", images)
	}
}

func TestAnalyzeSendsAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer srv.Close()

	c := New(time.Second, func() Endpoint { return Endpoint{URL: srv.URL, Model: "m", APIKey: "secret"} })
	if _, err := c.Analyze(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr func(error) bool
	}{
		{"http error", 503, "model loading", func(err error) bool {
			var se *StatusError
			return errors.As(err, &se) && se.Code == 503 && se.Body == "model loading"
		}},
		{"no choices", 200, `{"choices":[]}`, func(err error) bool { return errors.Is(err, ErrEmptyResponse) }},
		{"blank content", 200, `{"choices":[{"message":{"content":"  "}}]}`, func(err error) bool { return errors.Is(err, ErrEmptyResponse) }},
		{"bad json", 200, `not json`, func(err error) bool { return err != nil && strings.Contains(err.Error(), "decode response") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer srv.Close()
			c := New(time.Second, func() Endpoint { return Endpoint{URL: srv.URL, Model: "m"} })
			if _, err := c.Analyze(context.Background(), Request{}); !tt.wantErr(err) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUserPrompt(t *testing.T) {
	if p := userPrompt("Code", "   "); strings.Contains(p, "instructions") {
		t.Errorf("blank instructions leaked into prompt: %q", p)
	}
}
