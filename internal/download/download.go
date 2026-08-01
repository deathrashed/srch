package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Destination string
	Overwrite   bool
	Resume      bool
	ExpectedSHA string
	Timeout     time.Duration
	Progress    func(written, total int64)
}

type Result struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func Direct(ctx context.Context, rawURL string, options Options) (Result, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Result{}, fmt.Errorf("download requires an HTTP or HTTPS URL")
	}
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Minute
	}
	if options.Destination == "" {
		name := filepath.Base(u.Path)
		if name == "." || name == "/" || name == "" {
			name = "download"
		}
		options.Destination = sanitizeFilename(name)
	}
	if err := os.MkdirAll(filepath.Dir(options.Destination), 0o755); err != nil {
		return Result{}, fmt.Errorf("create download directory: %w", err)
	}
	if !options.Overwrite {
		if _, err := os.Stat(options.Destination); err == nil {
			return Result{}, fmt.Errorf("destination exists: %s", options.Destination)
		}
	}
	part := options.Destination + ".part"
	var offset int64
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if options.Resume {
		if info, statErr := os.Stat(part); statErr == nil {
			offset = info.Size()
			flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create download request: %w", err)
	}
	request.Header.Set("User-Agent", "srch/0.1")
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	client := &http.Client{Timeout: options.Timeout}
	response, err := client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{}, fmt.Errorf("download failed: %s", response.Status)
	}
	if offset > 0 && response.StatusCode != http.StatusPartialContent {
		offset = 0
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("open partial download: %w", err)
	}
	defer file.Close()
	total := response.ContentLength
	if total >= 0 {
		total += offset
	}
	written, err := copyProgress(file, response.Body, offset, total, options.Progress)
	if err != nil {
		return Result{}, fmt.Errorf("write download: %w", err)
	}
	if err := file.Sync(); err != nil {
		return Result{}, fmt.Errorf("sync download: %w", err)
	}
	if err := file.Close(); err != nil {
		return Result{}, fmt.Errorf("close download: %w", err)
	}
	digest, err := checksum(part)
	if err != nil {
		return Result{}, err
	}
	if options.ExpectedSHA != "" && !strings.EqualFold(options.ExpectedSHA, digest) {
		return Result{}, fmt.Errorf("checksum mismatch: expected %s, got %s", options.ExpectedSHA, digest)
	}
	if err := os.Rename(part, options.Destination); err != nil {
		return Result{}, fmt.Errorf("complete download: %w", err)
	}
	return Result{Path: options.Destination, Bytes: written + offset, SHA256: digest}, nil
}

func sanitizeFilename(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "\x00", "")
	value = strings.TrimSpace(replacer.Replace(value))
	if value == "" || value == "." || value == ".." {
		return "download"
	}
	return value
}

func copyProgress(destination io.Writer, source io.Reader, offset, total int64, progress func(int64, int64)) (int64, error) {
	buffer := make([]byte, 64*1024)
	var written int64
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			output, writeErr := destination.Write(buffer[:count])
			written += int64(output)
			if progress != nil {
				progress(offset+written, total)
			}
			if writeErr != nil {
				return written, writeErr
			}
			if output != count {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func checksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open download for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum download: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
