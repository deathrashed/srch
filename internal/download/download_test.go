package download_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"srch/internal/download"
)

func TestDirectDownloadAndChecksum(t *testing.T) {
	content := []byte("srch download fixture")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(content)
	}))
	defer server.Close()
	digest := sha256.Sum256(content)
	destination := filepath.Join(t.TempDir(), "fixture.txt")
	result, err := download.Direct(context.Background(), server.URL+"/fixture.txt", download.Options{
		Destination: destination,
		ExpectedSHA: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != destination || result.Bytes != int64(len(content)) {
		t.Fatalf("unexpected result: %#v", result)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestDirectRefusesOverwrite(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "existing")
	if err := os.WriteFile(destination, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := download.Direct(context.Background(), "https://example.com/file", download.Options{Destination: destination})
	if err == nil {
		t.Fatal("expected overwrite refusal")
	}
}
