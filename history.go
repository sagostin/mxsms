package main

import (
	"sync"
	"time"
)

type Item struct {
	Service string
	JID     string
	From    string
	To      string
	MsgID   int64
	SMSSeq  uint32
	Sended  time.Time
}

type History struct {
	list map[string]Item
	mu   sync.Mutex
}

func (h *History) Add(service, jid string, msgID int64, from, to string, seq uint32) {
	h.mu.Lock()
	if h.list == nil {
		h.list = make(map[string]Item)
	}
	h.list[to] = Item{
		Service: service,
		JID:     jid,
		From:    from,
		To:      to,
		MsgID:   msgID,
		SMSSeq:  seq,
		Sended:  time.Now(),
	}
	// log.Printf(">: %#v\n", h.list[to])
	h.mu.Unlock()
}

func (h *History) Get(from string) *Item {
	h.mu.Lock()
	item, ok := h.list[from]
	h.mu.Unlock()
	if !ok {
		return nil
	}
	return &item
}
