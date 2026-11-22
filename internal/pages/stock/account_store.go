package stock

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/selectbox"
	"transactions/internal/domain"
)

// accountStore is the workspace's stock accounts and the active one. All stock
// panels share it, so a switch on one tab is the same choice on every tab.
type accountStore struct {
	items  []domain.StockAccount
	ids    []string
	active string
}

func newAccountStore() *accountStore { return &accountStore{} }

// reset replaces the list; the current account stays selected when it still
// exists, otherwise the first account becomes active.
func (s *accountStore) reset(items []domain.StockAccount) {
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

func (s *accountStore) list() []domain.StockAccount { return s.items }
func (s *accountStore) idsOf() []string             { return s.ids }
func (s *accountStore) activeID() string            { return s.active }

func (s *accountStore) setActive(id string) bool {
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

func (s *accountStore) name(id string) string {
	for _, item := range s.items {
		if item.ID == id {
			return item.Name
		}
	}
	return id
}

// picker is the 账户 Select shared by the stock tabs. It reports whether the
// user changed the active account.
func (s *accountStore) picker(c ui.C) bool {
	if s == nil || len(s.ids) == 0 {
		return false
	}
	active := s.active
	if !selectbox.Select(c, &active, s.ids, s.name).Label("账户").Changed() {
		return false
	}
	return s.setActive(active)
}
