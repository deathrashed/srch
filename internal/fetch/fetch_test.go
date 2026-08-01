package fetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"srch/internal/fetch"
)

func TestGetAndFormatJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"srch","ok":true}`))
	}))
	defer server.Close()
	response, err := fetch.Get(context.Background(), server.URL, fetch.Options{Timeout: time.Second, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	body, err := fetch.Format(response, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "\n") || !strings.Contains(string(body), `"name": "srch"`) {
		t.Fatalf("unexpected formatted body: %s", body)
	}
}

func TestGetRejectsLargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("too large"))
	}))
	defer server.Close()
	_, err := fetch.Get(context.Background(), server.URL, fetch.Options{Timeout: time.Second, MaxBytes: 3})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}

func TestGetRejectsNonHTTP(t *testing.T) {
	_, err := fetch.Get(context.Background(), "file:///tmp/test", fetch.Options{})
	if err == nil {
		t.Fatal("expected scheme error")
	}
}
