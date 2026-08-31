package webui

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"

	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/pkg/consts"
)

//go:embed index.html
var indexHTML []byte

type Options struct {
	Host  string
	Port  int
	Token string
}

// Server wires HTTP handlers, task manager and child runner together.
type Server struct {
	opts    Options
	mgr     *Manager
	broker  *broker
	runner  *Runner
	mu      sync.Mutex
	running map[string]context.CancelFunc
	// queue serializes task execution: the bolt kv storage used by tdl
	// subprocesses can only be opened by a single process at a time.
	queue chan *Task
}

func New(opts Options) (*Server, error) {
	runner, err := NewRunner()
	if err != nil {
		return nil, err
	}
	return &Server{
		opts:    opts,
		mgr:     NewManager(),
		broker:  newBroker(),
		runner:  runner,
		running: map[string]context.CancelFunc{},
		queue:   make(chan *Task, 100),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.auth(s.handleIndex))
	mux.HandleFunc("/api/info", s.auth(s.handleInfo))
	mux.HandleFunc("/api/tasks", s.auth(s.handleTasks))
	mux.HandleFunc("/api/tasks/", s.auth(s.handleTask))
	mux.HandleFunc("/api/chats", s.auth(s.handleChats))
	mux.HandleFunc("/api/events", s.auth(s.handleEvents))
	mux.HandleFunc("/api/qr", s.auth(s.handleQR))

	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		s.CancelAll()
	}()

	go s.worker()

	logctx.From(ctx).Info("webui listening",
		zap.String("addr", "http://"+addr),
		zap.Bool("auth", s.opts.Token != ""))
	if s.opts.Token == "" {
		logctx.From(ctx).Warn("webui running without token authentication, " +
			"anyone who can reach this address can control tdl")
	}

	fmt.Printf("WebUI is running at http://%s\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// auth wraps a handler with optional bearer-token authentication.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Token != "" {
			token := r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token != s.opts.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

type infoResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Auth    bool   `json:"auth"`
	Addr    string `json:"addr"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, infoResponse{
		Version: consts.Version,
		Commit:  consts.Commit,
		Auth:    s.opts.Token != "",
		Addr:    fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port),
	})
}

// ---- task lifecycle ----

type createTaskRequest struct {
	Type   string         `json:"type"`
	Args   map[string]any `json:"args"`
	Global map[string]any `json:"global"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.mgr.List())
	case http.MethodPost:
		var req createTaskRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			http.Error(w, "task type is required", http.StatusBadRequest)
			return
		}
		task := &Task{
			ID:        uuid.NewString()[:8],
			Type:      req.Type,
			Args:      req.Args,
			Global:    req.Global,
			Status:    StatusPending,
			CreatedAt: time.Now(),
			LogTail:   []string{},
		}
		s.mgr.Add(task)
		s.broker.publish(Event{Type: "task-created", Data: task})
		select {
		case s.queue <- task:
		default:
			task.Status = StatusFailed
			task.Error = "task queue is full"
			task.EndedAt = time.Now()
			s.broker.publish(Event{Type: "task-state", Data: task})
		}
		writeJSON(w, http.StatusCreated, task)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	task, ok := s.mgr.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, task)
	case http.MethodDelete:
		s.cancelTask(id)
		writeJSON(w, http.StatusOK, task)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// worker pops tasks from the queue and runs them one at a time.
// tdl child processes share the bolt kv storage, which only allows a
// single process, so serial execution is required.
func (s *Server) worker() {
	for task := range s.queue {
		if task.Status == StatusCanceled {
			continue
		}
		s.runTask(task)
	}
}

// runTask spawns the child process, streams its output and updates the task.
func (s *Server) runTask(task *Task) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[task.ID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, task.ID)
		s.mu.Unlock()
	}()

	cmd, err := s.runner.Build(ctx, task)
	if err != nil {
		task.Status = StatusFailed
		task.Error = err.Error()
		task.EndedAt = time.Now()
		s.broker.publish(Event{Type: "task-state", Data: task})
		return
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	task.Status = StatusRunning
	task.StartedAt = time.Now()
	s.broker.publish(Event{Type: "task-state", Data: task})

	runErr := cmd.Run()

	task.EndedAt = time.Now()
	if ctx.Err() != nil {
		task.Status = StatusCanceled
	} else if runErr != nil {
		task.Status = StatusFailed
		if ee, ok := runErr.(*exec.ExitError); ok {
			task.ExitCode = ee.ExitCode()
		}
		task.Error = runErr.Error()
	} else {
		task.Status = StatusSucceeded
	}

	// ingest the full output into log buffer
	s.ingestOutput(task, buf.String())
	s.broker.publish(Event{Type: "task-state", Data: task})
}

// ingestOutput parses child output: splits lines, extracts progress and QR hints.
func (s *Server) ingestOutput(task *Task, output string) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)
	lines := []string{}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)

		if strings.HasPrefix(line, "WEBUI_QR:") {
			if task.QR == nil {
				task.QR = map[string]string{}
			}
			task.QR["url"] = strings.TrimPrefix(line, "WEBUI_QR:")
			s.broker.publish(Event{Type: "task-qr", Data: task})
		}
		if p := parseProgress(line); p != nil {
			task.Progress = p
			s.broker.publish(Event{Type: "task-progress", Data: task})
		}
	}
	task.LogTail = lines
	if len(lines) > 200 {
		task.LogTail = lines[len(lines)-200:]
	}
	s.broker.publish(Event{Type: "task-log", Data: task})
}

// CancelAll cancels every running or queued task.
func (s *Server) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cancel := range s.running {
		cancel()
		if t, ok := s.mgr.Get(id); ok {
			t.Status = StatusCanceled
			t.EndedAt = time.Now()
			s.broker.publish(Event{Type: "task-state", Data: t})
		}
	}
	for _, t := range s.mgr.List() {
		if t.Status == StatusPending {
			t.Status = StatusCanceled
			t.EndedAt = time.Now()
			s.broker.publish(Event{Type: "task-state", Data: t})
		}
	}
}

func (s *Server) cancelTask(id string) {
	t, found := s.mgr.Get(id)
	if !found {
		return
	}

	// finished tasks cannot be canceled anymore
	if t.Status != StatusPending && t.Status != StatusRunning {
		return
	}

	// still queued: just mark canceled, the worker will skip it
	if t.Status == StatusPending {
		t.Status = StatusCanceled
		t.EndedAt = time.Now()
		s.broker.publish(Event{Type: "task-state", Data: t})
		return
	}

	s.mu.Lock()
	cancel, ok := s.running[id]
	s.mu.Unlock()
	if ok {
		cancel()
	}
	t.Status = StatusCanceled
	t.EndedAt = time.Now()
	s.broker.publish(Event{Type: "task-state", Data: t})
}

// ---- chats (synchronous query) ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	ns := r.URL.Query().Get("ns")
	filter := r.URL.Query().Get("filter")
	argv := []string{}
	if ns != "" {
		argv = append(argv, "--ns", ns)
	}
	argv = append(argv, "chat", "ls", "--output", "json")
	if filter != "" {
		argv = append(argv, "--filter", filter)
	}

	cmd := exec.CommandContext(ctx, s.runner.bin, argv...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			http.Error(w, "chat ls failed: "+string(ee.Stderr), http.StatusBadGateway)
		} else {
			http.Error(w, "chat ls failed: "+err.Error(), http.StatusBadGateway)
		}
		return
	}
	var dialogs []any
	if err := json.Unmarshal(out, &dialogs); err != nil {
		http.Error(w, "parse chat list: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, dialogs)
}

// ---- QR image ----

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "url query parameter is required", http.StatusBadRequest)
		return
	}
	png, err := qrcode.Encode(url, qrcode.Medium, 320)
	if err != nil {
		http.Error(w, "encode qr: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

// ---- SSE ----

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.broker.subscribe()
	defer s.broker.unsubscribe(ch)

	// initial snapshot
	for _, t := range s.mgr.List() {
		s.writeSSE(w, flusher, Event{Type: "task-created", Data: t})
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			s.writeSSE(w, flusher, e)
		}
	}
}

func (s *Server) writeSSE(w http.ResponseWriter, f http.Flusher, e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
	f.Flush()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
