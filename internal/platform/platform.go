package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Runner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (ExecRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (ExecRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

type Services struct {
	GOOS   string
	Runner Runner
}

func New() Services {
	return Services{GOOS: runtime.GOOS, Runner: ExecRunner{}}
}

func (s Services) OpenURL(rawURL, browser string) error {
	switch s.GOOS {
	case "darwin":
		if browser != "" {
			return s.Runner.Run("open", "-a", browser, rawURL)
		}
		return s.Runner.Run("open", rawURL)
	case "windows":
		if browser != "" {
			return s.Runner.Run(browser, rawURL)
		}
		return s.Runner.Run("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		if browser != "" {
			return s.Runner.Run(browser, rawURL)
		}
		if _, err := s.Runner.LookPath("xdg-open"); err == nil {
			return s.Runner.Run("xdg-open", rawURL)
		}
		if _, err := s.Runner.LookPath("gio"); err == nil {
			return s.Runner.Run("gio", "open", rawURL)
		}
		return fmt.Errorf("no URL opener found; install xdg-utils or configure a browser")
	}
}

func (s Services) Copy(value string) error {
	var name string
	var args []string
	switch s.GOOS {
	case "darwin":
		name = "pbcopy"
	case "windows":
		name = "clip.exe"
	default:
		for _, candidate := range []string{"wl-copy", "xclip", "xsel"} {
			if _, err := s.Runner.LookPath(candidate); err == nil {
				name = candidate
				break
			}
		}
		if name == "xclip" {
			args = []string{"-selection", "clipboard"}
		} else if name == "xsel" {
			args = []string{"--clipboard", "--input"}
		}
	}
	if name == "" {
		return fmt.Errorf("no clipboard provider found")
	}
	if runner, ok := s.Runner.(ExecRunner); ok {
		_ = runner
		cmd := exec.Command(name, args...)
		cmd.Stdin = strings.NewReader(value)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("copy to clipboard: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil
	}
	return fmt.Errorf("clipboard input is unavailable through the configured command runner")
}

func (s Services) Available(name string) bool {
	_, err := s.Runner.LookPath(name)
	return err == nil
}
