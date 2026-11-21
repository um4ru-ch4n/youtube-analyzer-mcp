package summarizer

import (
	"strings"
	"testing"
)

func TestBuildPromptLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
		want     string
	}{
		{name: "known", language: "ru", want: "Write in Russian"},
		{name: "unknown", language: "", want: "same language as the original transcript"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildPrompt("title", "text", tt.language, 5)
			if !strings.Contains(got, tt.want) {
				t.Errorf("prompt does not contain %q:\n%s", tt.want, got)
			}
		})
	}
}
