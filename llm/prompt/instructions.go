package prompt

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
)

func LoadServerInstructions(path string) (string, error) {
	if text, err := loadServerInstructionsFromOS(path); err == nil && text != "" {
		return text, nil
	}
	return loadServerInstructionsFromFS(embeddedPromptFS, "server_instructions.md")
}

func loadServerInstructionsFromOS(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return normalizeInstructionText(string(data)), nil
}

func loadServerInstructionsFromFS(fsys fs.FS, path string) (string, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return "", fmt.Errorf("read embedded server instructions %s: %w", path, err)
	}
	return normalizeInstructionText(string(data)), nil
}

func normalizeInstructionText(text string) string {
	return strings.TrimSpace(text)
}
