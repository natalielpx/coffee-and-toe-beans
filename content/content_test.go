package content_test

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/game"
)

// If you edit a content file and something is wrong with it, these tests say
// which file and where.

func TestEveryJSONFileIsValid(t *testing.T) {
	err := fs.WalkDir(content.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := fs.ReadFile(content.FS, path)
		if err != nil {
			return err
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Errorf("%s is not valid JSON: %v (look for a missing comma or quote)", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheGameLoadsAllContent(t *testing.T) {
	g, err := game.New(game.Config{}, content.FS, time.Now())
	if err != nil {
		t.Fatalf("the game couldn't load its content: %v", err)
	}
	g.Resize(game.FrameW, game.FrameH)
	if g.View() == "" {
		t.Fatal("nothing was drawn")
	}
}
