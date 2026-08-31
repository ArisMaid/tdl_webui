package webui

import (
	"strings"
	"testing"
)

func TestParseProgress(t *testing.T) {
	tests := []struct {
		line string
		want *Progress
	}{
		{
			line: " 50.0% | 10.5 MB/s | ETA: 10s | 2/4",
			want: &Progress{Percent: 50, Speed: "10.5 MB/s", ETA: "10s", Done: 2, Total: 4},
		},
		{
			line: "done 100.0%",
			want: &Progress{Percent: 100},
		},
		{
			line: "no progress here",
			want: nil,
		},
		{
			line: "",
			want: nil,
		},
	}
	for _, tt := range tests {
		got := parseProgress(tt.line)
		if tt.want == nil {
			if got != nil {
				t.Errorf("parseProgress(%q) = %+v, want nil", tt.line, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("parseProgress(%q) = nil, want %+v", tt.line, tt.want)
			continue
		}
		if got.Percent != tt.want.Percent || got.Speed != tt.want.Speed ||
			got.ETA != tt.want.ETA || got.Done != tt.want.Done || got.Total != tt.want.Total {
			t.Errorf("parseProgress(%q) = %+v, want %+v", tt.line, got, tt.want)
		}
	}
}

func TestRingBuffer(t *testing.T) {
	rb := newRingBuffer(3)
	rb.Append([]string{"a", "b"})
	rb.Append([]string{"c", "d"})
	got := rb.All()
	if strings.Join(got, ",") != "b,c,d" {
		t.Errorf("ring buffer = %v, want [b c d]", got)
	}
}

func TestAsStrings(t *testing.T) {
	// elements with spaces must be preserved as single argv entries
	if got := asStrings([]any{"a", "b c", 1, " d "}); strings.Join(got, "|") != "a|b c|d" {
		t.Errorf("asStrings = %v", got)
	}
}

func TestBuildCommandInjection(t *testing.T) {
	// values containing shell metacharacters must be passed as single
	// argv entries, never through a shell.
	r := &Runner{bin: "tdl"}
	task := &Task{
		Type:   "download",
		Args:   map[string]any{"urls": []any{"https://t.me/x/1&rm -rf /;echo pwned"}},
		Global: map[string]any{"ns": "default"},
	}
	cmd, err := r.Build(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args, "\x00")
	if !strings.Contains(joined, "https://t.me/x/1&rm -rf /;echo pwned") {
		t.Errorf("url not preserved in argv: %q", joined)
	}
	// the url must be exactly one argv element
	found := false
	for _, a := range cmd.Args {
		if a == "https://t.me/x/1&rm -rf /;echo pwned" {
			found = true
		}
	}
	if !found {
		t.Errorf("url not a single argv element: %q", cmd.Args)
	}
}

func TestBuildGlobalArgs(t *testing.T) {
	r := &Runner{bin: "tdl"}
	task := &Task{
		Type:   "version",
		Global: map[string]any{"ns": "my-ns", "threads": 8.0, "debug": true},
	}
	cmd, err := r.Build(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(cmd.Args, " ") + " "
	for _, want := range []string{" --ns my-ns ", " --threads 8 ", " --debug ", " --disable-progress-ps "} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv %q missing %q", cmd.Args, want)
		}
	}
}
