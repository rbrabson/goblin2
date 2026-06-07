package message

import (
	"log/slog"
	"sync"
	"time"
)

var (
	manager = newManager()
)

// paginatorManager is the main controller for the paginator. It contains all
// active paginators.
type paginatorManager struct {
	mutex      sync.Mutex
	paginators map[string]*Paginator
}

// newManager creates a new paginator manager. The manager starts a goroutine
// to clean up expired paginators. The cleanup goroutine runs every minute.
func newManager() *paginatorManager {
	manager := &paginatorManager{
		paginators: map[string]*Paginator{},
		mutex:      sync.Mutex{},
	}
	manager.startCleanup()

	return manager
}

// addPaginator adds a paginator to the manager.
func (m *paginatorManager) addPaginator(paginator *Paginator) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.paginators[paginator.id] = paginator
	slog.Debug("added paginator to manager",
		slog.String("paginator", paginator.id),
		slog.Int("count", len(m.paginators)),
	)
}

// removePaginator removes a paginator from the manager.
func (m *paginatorManager) removePaginator(p *Paginator) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.paginators, p.id)
	slog.Debug("removed paginator from manager",
		slog.String("paginator", p.id),
	)
}

// cleanup removes expired paginated messages from all paginators.
func (m *paginatorManager) cleanup() {
	m.mutex.Lock()
	paginators := make([]*Paginator, 0, len(m.paginators))
	for _, p := range m.paginators {
		paginators = append(paginators, p)
	}
	m.mutex.Unlock()

	for _, p := range paginators {
		p.cleanup()
	}
}

// startCleanup starts a goroutine that cleans up expired paginated messages every minute.
func (m *paginatorManager) startCleanup() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			m.cleanup()
		}
	}()
}
