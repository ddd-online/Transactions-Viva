// Package ledger is the 记账 section's ledger list and its shared active ledger.
// The records, tags and templates pages all paint the same 账本 Select and read
// the same active id while the application runs. The list is loaded from the
// workspace by accounting.SetRepository.
package ledger

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/selectbox"
	"transactions/internal/domain"
)

// Store is the workspace's ledgers and the active one. Every accounting page
// holds the same Store, so switching a ledger on one page changes it for all.
type Store struct {
	items  []domain.Ledger
	ids    []string
	active string
}

// NewStore returns an empty ledger list; accounting fills it from the
// workspace repository.
func NewStore() *Store { return &Store{} }

// Reset replaces the list with the workspace's ledgers. The active ledger stays
// selected when it still exists, otherwise the first ledger becomes active.
func (s *Store) Reset(items []domain.Ledger) {
	s.items = append(s.items[:0], items...)
	s.ids = s.ids[:0]
	for _, item := range s.items {
		s.ids = append(s.ids, item.ID)
	}
	active := s.active
	found := false
	for _, item := range s.items {
		if item.ID == active {
			found = true
			break
		}
	}
	if found {
		return
	}
	if len(s.items) == 0 {
		s.active = ""
		return
	}
	s.active = s.items[0].ID
}

// List is the ledgers in the order the selectors show them.
func (s *Store) List() []domain.Ledger { return s.items }

// IDs are the ledgers' ids in the same order, ready for a Select.
func (s *Store) IDs() []string { return s.ids }

// Active is the id of the ledger the accounting pages show.
func (s *Store) Active() string { return s.active }

// SetActive chooses a ledger and reports whether the active one changed.
func (s *Store) SetActive(id string) bool {
	if id == s.active {
		return false
	}
	for _, item := range s.items {
		if item.ID == id {
			s.active = id
			return true
		}
	}
	return false
}

// Name is the ledger's display name; an unknown id is shown as written.
func (s *Store) Name(id string) string {
	for _, item := range s.items {
		if item.ID == id {
			return item.Name
		}
	}
	return id
}

// Picker is the 账本 Select shared by the accounting pages, at the leading edge
// of their toolbar. It reports whether the user changed the active ledger.
func Picker(c ui.C, store *Store) bool {
	if store == nil || len(store.ids) == 0 {
		return false
	}
	active := store.Active()
	if !selectbox.Select(c, &active, store.ids, store.Name).Label("账本").Changed() {
		return false
	}
	return store.SetActive(active)
}
