package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type Options struct {
	Timeout  time.Duration
	MaxBytes int64
	Headers  map[string]string
	Backend  string
}

type Response struct {
	URL        string              `json:"url"`
	Status     string              `json:"status"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	MediaType  string              `json:"media_type"`
	Body       []byte              `json:"-"`
	Duration   time.Duration       `json:"duration"`
}

func Get(ctx context.Context, rawURL string, options Options) (Response, error) {
	if _, err := validateURL(rawURL); err != nil {
		return Response{}, err
	}
	if options.Backend == "curl" {
		return curlGet(ctx, rawURL, options)
	}
	if options.Timeout <= 0 {
		options.Timeout = 20 * time.Second
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 10 << 20
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("User-Agent", "srch/0.1")
	for key, value := range options.Headers {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: options.Timeout}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, options.MaxBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > options.MaxBytes {
		return Response{}, fmt.Errorf("response exceeds %d byte limit", options.MaxBytes)
	}
	return Response{
		URL:        response.Request.URL.String(),
		Status:     response.Status,
		StatusCode: response.StatusCode,
		Headers:    response.Header.Clone(),
		MediaType:  response.Header.Get("Content-Type"),
		Body:       body,
		Duration:   time.Since(started),
	}, nil
}

func Format(response Response, mode string) ([]byte, error) {
	if mode == "json" || strings.Contains(response.MediaType, "json") {
		var value any
		if err := json.Unmarshal(response.Body, &value); err == nil {
			return json.MarshalIndent(value, "", "  ")
		}
	}
	return response.Body, nil
}

func CurlCommand(rawURL string, headers map[string]string) []string {
	args := []string{"curl", "--fail-with-body", "--location", "--silent", "--show-error"}
	for key, value := range headers {
		args = append(args, "--header", key+": "+value)
	}
	return append(args, rawURL)
}

func validateURL(rawURL string) (*url.URL, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	return u, nil
}

func curlGet(ctx context.Context, rawURL string, options Options) (Response, error) {
	args := CurlCommand(rawURL, options.Headers)
	started := time.Now()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	body, err := cmd.Output()
	if err != nil {
		return Response{}, fmt.Errorf("curl fetch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return Response{URL: rawURL, Status: "curl", StatusCode: http.StatusOK, Body: body, Duration: time.Since(started)}, nil
}
