package model

import "sync"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "downloading"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Download struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	FileName   string  `json:"fileName"`
	Path       string  `json:"path"`
	Status     Status  `json:"status"`
	Progress   float64 `json:"progress"` // 0.0 to 100.0
	Error      string  `json:"error,omitempty"`
	TotalBytes int64   `json:"totalBytes,omitempty"`
	DoneBytes  int64   `json:"doneBytes,omitempty"`
}

type Store struct {
	mu        sync.RWMutex
	downloads map[string]*Download
}

func NewStore() *Store {
	return &Store{
		downloads: make(map[string]*Download),
	}
}

func (s *Store) Add(dl *Download) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.downloads[dl.ID] = dl
}

func (s *Store) GetAll() []*Download {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Download, 0, len(s.downloads))
	for _, v := range s.downloads {
		list = append(list, v)
	}
	return list
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.downloads)
}

func (s *Store) Get(id string) (*Download, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dl, exists := s.downloads[id]
	return dl, exists
}
