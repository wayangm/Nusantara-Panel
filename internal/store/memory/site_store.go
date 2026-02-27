package memory

import (
	"context"
	"sync"

	"nusantara/internal/store"
)

type SiteStore struct {
	mu    sync.RWMutex
	sites []store.Site
}

func NewSiteStore() *SiteStore {
	return &SiteStore{
		sites: make([]store.Site, 0, 32),
	}
}

func (s *SiteStore) CreateSite(_ context.Context, site store.Site) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sites = append(s.sites, site)
	return nil
}

func (s *SiteStore) ListSites(_ context.Context) ([]store.Site, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]store.Site, len(s.sites))
	copy(out, s.sites)
	return out, nil
}

