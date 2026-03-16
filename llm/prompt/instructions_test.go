package prompt

import "testing"

func TestLoadServerInstructions(t *testing.T) {
	text, err := LoadServerInstructions("server_instructions.md")
	if err != nil {
		t.Fatalf("LoadServerInstructions() error = %v", err)
	}
	if text == "" {
		t.Fatal("LoadServerInstructions() returned empty text")
	}
	if !containsAll(text, "你是 hades 的交易分析与执行助手", "get_positions", "get_quote") {
		t.Fatalf("LoadServerInstructions() text = %q, want key guidance", text)
	}
}
