package webui

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusSucceeded TaskStatus = "succeeded"
	StatusFailed    TaskStatus = "failed"
	StatusCanceled  TaskStatus = "canceled"
)

// Progress is the parsed progress information from task output.
type Progress struct {
	Percent float64 `json:"percent"` // 0-100
	Speed   string  `json:"speed,omitempty"`
	Done    int     `json:"done,omitempty"`
	Total   int     `json:"total,omitempty"`
	ETA     string  `json:"eta,omitempty"`
	Current string  `json:"current,omitempty"`
}

type Task struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Args      map[string]any    `json:"args"`
	Global    map[string]any    `json:"global"`
	Status    TaskStatus        `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	StartedAt time.Time         `json:"startedAt,omitempty"`
	EndedAt   time.Time         `json:"endedAt,omitempty"`
	ExitCode  int               `json:"exitCode,omitempty"`
	Error     string            `json:"error,omitempty"`
	Progress  *Progress         `json:"progress,omitempty"`
	LogTail   []string          `json:"logTail"`
	QR        map[string]string `json:"qr,omitempty"`
}

// ringBuffer keeps the last N log lines.
type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

func (r *ringBuffer) Append(lines []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, lines...)
	if len(r.lines) > r.max {
		r.lines = r.lines[len(r.lines)-r.max:]
	}
}

func (r *ringBuffer) All() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// Manager holds in-memory tasks.
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string
}

func NewManager() *Manager {
	return &Manager{tasks: map[string]*Task{}, order: []string{}}
}

func (m *Manager) Add(t *Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[t.ID] = t
	m.order = append(m.order, t.ID)
}

func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

// List returns tasks newest first.
func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Task, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		out = append(out, m.tasks[m.order[i]])
	}
	return out
}

// parseProgress extracts progress information from a line such as:
// " 50.0% | 10.5 MB/s | ETA: 10s | 2/4"
func parseProgress(line string) *Progress {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	p := &Progress{}
	has := false

	if i := strings.Index(line, "%"); i > 0 {
		numStr := strings.TrimSpace(line[:i])
		// take the last numeric token before %
		if j := strings.LastIndexAny(numStr, " 	|"); j >= 0 {
			numStr = numStr[j+1:]
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64); err == nil {
			p.Percent = f
			has = true
		}
	}

	for _, field := range strings.Split(line, "|") {
		field = strings.TrimSpace(field)
		switch {
		case strings.HasPrefix(field, "ETA"):
			p.ETA = strings.TrimSpace(strings.TrimPrefix(field, "ETA"))
			p.ETA = strings.TrimPrefix(p.ETA, ":")
			p.ETA = strings.TrimSpace(p.ETA)
			has = true
		case strings.Contains(field, "/s"):
			p.Speed = field
			has = true
		case strings.Contains(field, "/"):
			parts := strings.SplitN(field, "/", 2)
			if d, err1 := strconv.Atoi(strings.TrimSpace(parts[0])); err1 == nil {
				if t, err2 := strconv.Atoi(strings.TrimSpace(parts[1])); err2 == nil {
					p.Done, p.Total = d, t
					has = true
				}
			}
		}
	}

	if !has {
		return nil
	}
	return p
}
