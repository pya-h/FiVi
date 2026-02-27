package cleanup

import (
	"sync"

	"fivi/internal/logx"
)

// Manager tracks cleanup functions and ensures they run exactly once.
type Manager struct {
	mu    sync.Mutex
	once  sync.Once
	hooks []hook
}

type hook struct {
	name string
	fn   func()
}

// New creates a new cleanup manager.
func New() *Manager {
	return &Manager{}
}

// Register adds a cleanup function. Functions run in reverse registration order
// (LIFO) to mirror defer semantics — last registered runs first.
func (m *Manager) Register(name string, fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook{name: name, fn: fn})
}

// Run executes all registered cleanup functions exactly once, in reverse order.
// Safe to call multiple times — only the first invocation runs the hooks.
func (m *Manager) Run() {
	m.once.Do(func() {
		m.mu.Lock()
		hooks := make([]hook, len(m.hooks))
		copy(hooks, m.hooks)
		m.mu.Unlock()

		for i := len(hooks) - 1; i >= 0; i-- {
			h := hooks[i]
			func() {
				defer func() {
					if r := recover(); r != nil {
						logx.Errorf("cleanup panic in %q: %v", h.name, r)
					}
				}()
				logx.Infof("cleanup: %s", h.name)
				h.fn()
			}()
		}
	})
}
