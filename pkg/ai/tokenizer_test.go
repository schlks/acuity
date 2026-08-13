package ai_test

import (
	"acuity/pkg/ai"
	"testing"
)

func TestCLIPTokenizer(t *testing.T) {
	tok, err := ai.NewCLIPTokenizer()
	if err != nil {
		t.Fatalf("Failed to initialize tokenizer: %v", err)
	}

	tokens, err := tok.Encode("a photo of a dog")
	if err != nil {
		t.Fatalf("Failed to encode text: %v", err)
	}

	if len(tokens) == 0 {
		t.Fatalf("Expected non-empty tokens, got 0")
	}

	t.Logf("Successfully encoded 'a photo of a dog': %v", tokens)
}
