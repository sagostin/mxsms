package main

import (
	"sync"
	"time"
)

type historyItem struct {
	MXName string    // название MX-сервера
	JID    string    // уникальный идентификатор пользователя
	Sended time.Time // время добавления записи
}

type History struct {
	list map[string]map[string]historyItem // to|from map
	mu   sync.RWMutex
}

func (h *History) Add(mxName, jid, from, to string) {
	h.mu.Lock()
	if h.list == nil {
		h.list = make(map[string]map[string]historyItem)
	}
	items := h.list[to]
	if items == nil {
		items = make(map[string]historyItem)
		h.list[to] = items
	}
	items[from] = historyItem{
		MXName: mxName,
		JID:    jid,
		Sended: time.Now(),
	}
	h.mu.Unlock()
}

func (h *History) Get(from, to string) (mxName, jid string) {
	h.mu.RLock()
	items := h.list[to]
	if items == nil {
		h.mu.RUnlock()
		return
	}
	item := items[from]
	h.mu.RUnlock()
	return item.MXName, item.JID
}

func (h *History) GetFrom(froms []string, to string) (from string) {
	h.mu.RLock()
	items := h.list[to]
	if items == nil {
		h.mu.RUnlock()
		return froms[0]
	}
	var (
		sended   time.Time
		sendFrom string
	)
	for _, from := range froms {
		item, ok := items[from]
		if !ok {
			h.mu.RUnlock()
			return from
		}
		if sended.IsZero() || item.Sended.Before(sended) {
			sended = item.Sended
			sendFrom = from
		}
	}
	h.mu.RUnlock()
	return sendFrom
}
