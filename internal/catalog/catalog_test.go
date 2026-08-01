package catalog_test

import (
	"path/filepath"
	"testing"

	"srch/internal/catalog"
)

func TestEmbeddedCatalogueIsValid(t *testing.T) {
	cat, err := catalog.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cat.Engines("", true)); got < 70 {
		t.Fatalf("expected a broad initial catalogue, got %d engines", got)
	}
	for _, category := range []string{"web", "images", "ai", "music", "lyrics", "code", "research", "video", "shopping"} {
		if got := len(cat.Engines(category, false)); got == 0 {
			t.Fatalf("category %q has no enabled engines", category)
		}
	}
}
