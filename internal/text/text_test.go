package text

import (
	"strings"
	"testing"
	"testing/fstest"

	"coffee-and-toe-beans/content"
)

func TestLoadUI(t *testing.T) {
	u, err := LoadUI(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	if u.Title == "" || len(u.TooSmall) == 0 {
		t.Fatal("ui text incomplete")
	}
}

func TestMissingTextIsReported(t *testing.T) {
	fsys := fstest.MapFS{"text/ui.json": {Data: []byte(`{"title": "x"}`)}}
	_, err := LoadUI(fsys)
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("expected a message naming the missing key, got %v", err)
	}
}

func TestFill(t *testing.T) {
	got := Fill("at least {need} (now {have})", map[string]string{"need": "104×42", "have": "70×20"})
	if got != "at least 104×42 (now 70×20)" {
		t.Fatal(got)
	}
}
