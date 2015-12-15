package main

import (
	"log"

	"github.com/fiorix/go-smpp/smpp"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
)

type SMS struct {
	tx *smpp.Transceiver
}

func NewSMS(addr, user, password string) *SMS {
	tx := &smpp.Transceiver{
		Addr:   addr,
		User:   user,
		Passwd: password,
	}
	sms := &SMS{
		tx: tx,
	}
	tx.Handler = sms.transceiverHandler
	conn := tx.Bind() // make persistent connection.
	go func() {
		for c := range conn {
			log.Println("SMPP connection status:", c.Status())
		}
	}()
	return sms
}

func (s *SMS) Close() error {
	return s.tx.Close()
}

func (s *SMS) transceiverHandler(p pdu.Body) {
	switch p.Header().ID {
	case pdu.DeliverSMID:
		f := p.Fields()
		src := f[pdufield.SourceAddr]
		dst := f[pdufield.DestinationAddr]
		txt := f[pdufield.ShortMessage]
		log.Printf("Short message from=%q to=%q: %q", src, dst, txt)
	}
}

func (s *SMS) Send(from, to, msg string) (msgID string, err error) {
	sm, err := s.tx.Submit(&smpp.ShortMessage{
		Src:      from,
		Dst:      to,
		Text:     pdutext.Raw(msg),
		Register: smpp.FinalDeliveryReceipt,
	})
	if err == nil {
		msgID = sm.RespID()
	}
	return
}
