package context

import (
	"sync"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var _ app.ContextStore = (*Memory)(nil)

type Memory struct {
	mu      sync.Mutex
	current app.RecentContext
	has     bool
}

func NewMemory() *Memory { return &Memory{} }

func (m *Memory) Replace(ctx app.RecentContext) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = cloneContext(ctx)
	m.has = true
}

func (m *Memory) Current() (app.RecentContext, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.has {
		return app.RecentContext{}, false
	}
	return cloneContext(m.current), true
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = app.RecentContext{}
	m.has = false
}

func cloneContext(ctx app.RecentContext) app.RecentContext {
	ctx.Summary = cloneSummary(ctx.Summary)
	return ctx
}

func cloneSummary(summary app.Summary) app.Summary {
	if summary == nil {
		return nil
	}
	clone := make(app.Summary, len(summary))
	for language, text := range summary {
		clone[language] = text
	}
	return clone
}
