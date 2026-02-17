package cli

import "testing"

func TestNormalizeInteractiveInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "LF", input: "hello\n", want: "hello"},
		{name: "CRLF", input: "hello\r\n", want: "hello"},
		{name: "NoNewline", input: "hello", want: "hello"},
		{name: "OnlyNewline", input: "\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeInteractiveInput(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeInteractiveInput(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsExitCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "exit", want: true},
		{input: "EXIT", want: true},
		{input: " quit ", want: true},
		{input: "quit", want: true},
		{input: "help", want: false},
		{input: "", want: false},
	}

	for _, tt := range tests {
		got := isExitCommand(tt.input)
		if got != tt.want {
			t.Fatalf("isExitCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolveCLIChannel(t *testing.T) {
	if got := resolveCLIChannel(true); got != "cli" {
		t.Fatalf("resolveCLIChannel(true) = %q, want %q", got, "cli")
	}
	if got := resolveCLIChannel(false); got != "cli_plain" {
		t.Fatalf("resolveCLIChannel(false) = %q, want %q", got, "cli_plain")
	}
}

func TestStripMarkdown(t *testing.T) {
	input := "## Title\nUse **bold** and [link](https://example.com) with `code`."
	got := stripMarkdown(input)
	want := "Title\nUse bold and link (https://example.com) with code."
	if got != want {
		t.Fatalf("stripMarkdown() = %q, want %q", got, want)
	}
}
