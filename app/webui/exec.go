package webui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-faster/errors"
	"go.uber.org/zap"

	"github.com/iyear/tdl/core/logctx"
)

// Runner executes tdl child processes for tasks.
type Runner struct {
	bin string // path to the tdl executable
}

func NewRunner() (*Runner, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "get executable path")
	}
	return &Runner{bin: exe}, nil
}

// asStrings converts an []any of strings into []string.
// Every element is kept intact as a single argv entry so that URLs and
// file paths containing spaces survive; splitting is the frontend's job.
func asStrings(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asInt(v any) int {
	return int(asFloat(v))
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(n), "%f", &f)
		return f
	}
	return 0
}

// Build creates the exec.Cmd for a task.
// t.Type, t.Args (options) and t.Global drive the command construction.
func (r *Runner) Build(ctx context.Context, t *Task) (*exec.Cmd, error) {
	global := t.Global
	if global == nil {
		global = map[string]any{}
	}
	opts := t.Args
	if opts == nil {
		opts = map[string]any{}
	}

	argv := []string{}
	argv = append(argv, r.globalArgs(global)...)

	switch t.Type {
	case "download":
		argv = append(argv, "download")
		argv = append(argv, r.downloadArgs(opts)...)
	case "upload":
		argv = append(argv, "upload")
		argv = append(argv, r.uploadArgs(opts)...)
	case "forward":
		argv = append(argv, "forward")
		argv = append(argv, r.forwardArgs(opts)...)
	case "chat-list":
		argv = append(argv, "chat", "ls", "--output", "json")
		if f := asString(opts["filter"]); f != "" && f != "true" {
			argv = append(argv, "--filter", f)
		}
	case "login-qr":
		argv = append(argv, "login", "--type", "qr", "--webui-qr")
	case "version":
		argv = append(argv, "version")
	default:
		return nil, errors.Errorf("unknown task type: %s", t.Type)
	}

	logctx.From(ctx).Debug("build child command", zap.Strings("argv", argv))

	cmd := exec.CommandContext(ctx, r.bin, argv...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TDL_WEBUI_CHILD=1")
	cmd.Stdin = strings.NewReader("") // disable survey interaction
	return cmd, nil
}

// globalArgs builds the persistent flags shared by all subcommands.
func (r *Runner) globalArgs(g map[string]any) []string {
	args := []string{}
	if s := asString(g["ns"]); s != "" {
		args = append(args, "--ns", s)
	}
	if s := asString(g["proxy"]); s != "" {
		args = append(args, "--proxy", s)
	}
	if s := asString(g["ntp"]); s != "" {
		args = append(args, "--ntp", s)
	}
	if asBool(g["debug"]) {
		args = append(args, "--debug")
	}
	if n := asInt(g["threads"]); n > 0 {
		args = append(args, "--threads", fmt.Sprint(n))
	}
	if n := asInt(g["limit"]); n > 0 {
		args = append(args, "--limit", fmt.Sprint(n))
	}
	if n := asInt(g["pool"]); n > 0 {
		args = append(args, "--pool", fmt.Sprint(n))
	}
	if s := asString(g["delay"]); s != "" {
		args = append(args, "--delay", s)
	}
	if s := asString(g["reconnect-timeout"]); s != "" {
		args = append(args, "--reconnect-timeout", s)
	}
	// always disable terminal progress renderer in child processes
	args = append(args, "--disable-progress-ps")
	return args
}

func (r *Runner) downloadArgs(o map[string]any) []string {
	args := []string{}
	for _, u := range asStrings(o["urls"]) {
		args = append(args, "--url", u)
	}
	for _, f := range asStrings(o["files"]) {
		args = append(args, "--file", f)
	}
	if s := asString(o["template"]); s != "" {
		args = append(args, "--template", s)
	}
	for _, i := range asStrings(o["include"]) {
		args = append(args, "--include", i)
	}
	for _, e := range asStrings(o["exclude"]) {
		args = append(args, "--exclude", e)
	}
	if s := asString(o["dir"]); s != "" {
		args = append(args, "--dir", s)
	}
	boolFlags := map[string]string{
		"rewrite-ext": "--rewrite-ext",
		"skip-same":   "--skip-same",
		"desc":        "--desc",
		"takeout":     "--takeout",
		"group":       "--group",
		"continue":    "--continue",
		"restart":     "--restart",
	}
	for k, flag := range boolFlags {
		if asBool(o[k]) {
			args = append(args, flag)
		}
	}
	return args
}

func (r *Runner) uploadArgs(o map[string]any) []string {
	args := []string{}
	if s := asString(o["chat"]); s != "" {
		args = append(args, "--chat", s)
	}
	if n := asInt(o["topic"]); n != 0 {
		args = append(args, "--topic", fmt.Sprint(n))
	}
	if s := asString(o["to"]); s != "" {
		args = append(args, "--to", s)
	}
	for _, p := range asStrings(o["paths"]) {
		args = append(args, "--path", p)
	}
	for _, i := range asStrings(o["include"]) {
		args = append(args, "--include", i)
	}
	for _, e := range asStrings(o["exclude"]) {
		args = append(args, "--exclude", e)
	}
	if s := asString(o["caption"]); s != "" {
		args = append(args, "--caption", s)
	}
	if asBool(o["rm"]) {
		args = append(args, "--rm")
	}
	if asBool(o["photo"]) {
		args = append(args, "--photo")
	}
	return args
}

func (r *Runner) forwardArgs(o map[string]any) []string {
	args := []string{}
	for _, f := range asStrings(o["from"]) {
		args = append(args, "--from", f)
	}
	if s := asString(o["to"]); s != "" {
		args = append(args, "--to", s)
	}
	if s := asString(o["edit"]); s != "" {
		args = append(args, "--edit", s)
	}
	if s := asString(o["mode"]); s != "" {
		args = append(args, "--mode", s)
	}
	boolFlags := map[string]string{
		"silent":  "--silent",
		"dry-run": "--dry-run",
		"single":  "--single",
		"desc":    "--desc",
	}
	for k, flag := range boolFlags {
		if asBool(o[k]) {
			args = append(args, flag)
		}
	}
	return args
}
