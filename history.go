package main

import (
	"sync"
	"time"
)

type historyItem struct {
	MXName string    // MX server name
	JID    string    // unique user identifier
	Sended time.Time // record addition time
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

func (h *History) GetFrom(froms map[string]string, to, jid string) (from string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	items := h.list[to]
	if items == nil {
		// Return the first phone number from the map if no history
		for number := range froms {
			return number
		}
	}
	for from, item := range items {
		if item.JID == jid {
			return from
		}
	}
	var (
		sended   time.Time
		sendFrom string
	)
	for number := range froms {
		item, ok := items[number]
		if !ok {
			return number
		}
		if sended.IsZero() || item.Sended.Before(sended) {
			sended = item.Sended
			sendFrom = number
		}
	}
	return sendFrom
}
