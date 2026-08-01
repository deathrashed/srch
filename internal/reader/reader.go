package reader

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/glamour/v2"
)

func Extract(ctx context.Context, rawURL string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "defuddle", "parse", rawURL, "--md")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	content, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("defuddle extraction: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return content, nil
}

func Render(markdown []byte, width int, style string) (string, error) {
	if width <= 0 {
		width = 80
	}
	if style == "" || style == "auto" {
		style = "dark"
	}
	renderer, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(width))
	if err != nil {
		return "", fmt.Errorf("create markdown renderer: %w", err)
	}
	result, err := renderer.RenderBytes(markdown)
	if err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return string(result), nil
}
