package mcp

import (
	"strings"
	"testing"
)

func TestNormalizeCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantLines   int
		wantTabbed  string // подстрока, которая должна присутствовать с табами
		wantErr     bool
		errContains string
	}{
		{
			name: "Cookie-Editor format with 4 spaces",
			input: "# Netscape HTTP Cookie File\n" +
				"# Comment\n" +
				".youtube.com    TRUE    /    TRUE    1812787525    SAPISID    abc123\n" +
				"#HttpOnly_.youtube.com    TRUE    /    TRUE    1812787525    SSID    xyz789\n",
			wantLines:  2,
			wantTabbed: ".youtube.com\tTRUE\t/\tTRUE\t1812787525\tSAPISID\tabc123",
		},
		{
			name: "уже tab-separated",
			input: "# Netscape HTTP Cookie File\n" +
				".youtube.com\tTRUE\t/\tTRUE\t1812787525\tSAPISID\tabc123\n",
			wantLines:  1,
			wantTabbed: ".youtube.com\tTRUE\t/\tTRUE\t1812787525\tSAPISID\tabc123",
		},
		{
			name:        "пустой input",
			input:       "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "только комментарии без cookies",
			input:       "# Netscape HTTP Cookie File\n# nothing else\n",
			wantErr:     true,
			errContains: "no cookie lines",
		},
		{
			name: "сломанное число полей",
			input: "# Netscape HTTP Cookie File\n" +
				".youtube.com    TRUE    /    SAPISID    abc123\n",
			wantErr:     true,
			errContains: "expected 7",
		},
		{
			name: "значение с пробелом внутри (одиночный)",
			input: "# Netscape HTTP Cookie File\n" +
				".youtube.com    TRUE    /    TRUE    1812787525    PREF    a=1 b=2\n",
			wantLines:  1,
			wantTabbed: ".youtube.com\tTRUE\t/\tTRUE\t1812787525\tPREF\ta=1 b=2",
		},
		{
			name: "автодобавление заголовка",
			input: ".youtube.com    TRUE    /    TRUE    1812787525    SAPISID    abc123\n",
			wantLines:  1,
			wantTabbed: "# Netscape HTTP Cookie File\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, lines, err := normalizeCookies(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lines != tt.wantLines {
				t.Errorf("lines = %d, want %d", lines, tt.wantLines)
			}
			if !strings.Contains(got, tt.wantTabbed) {
				t.Errorf("output does not contain %q\ngot:\n%s", tt.wantTabbed, got)
			}
		})
	}
}
