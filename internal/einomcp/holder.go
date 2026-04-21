package einomcp

import "sync"

// English note.
type ConversationHolder struct {
	mu sync.RWMutex
	id string
}

func (h *ConversationHolder) Set(id string) {
	h.mu.Lock()
	h.id = id
	h.mu.Unlock()
}

func (h *ConversationHolder) Get() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.id
}
