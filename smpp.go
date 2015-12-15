package main

import (
	"log"
	"sync"

	"github.com/fiorix/go-smpp/smpp"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
)

type SMS struct {
	tx     *smpp.Transceiver
	status smpp.ConnStatus
	mu     sync.RWMutex
}

func NewSMS(addr, user, password string) (sms *SMS) {
	return &SMS{
		tx: &smpp.Transceiver{
			Addr:       addr,
			User:       user,
			Passwd:     password,
			SystemType: "SMPP", // именно заглавными буквами
			Handler:    sms.handler,
		},
	}
}

func (sms *SMS) Bind() (err error) {
	conn := sms.tx.Bind() // подключаемся
	sms.status = <-conn   // получаем статус подключения
	switch sms.status.Status() {
	case smpp.Connected: // подключение успешно
	default: // ошибка подключения
		err = sms.status.Error()
	}
	go func() { // запускаем отслеживание состояния подключения
		for status := range conn {
			sms.mu.Lock()
			sms.status = status
			sms.mu.Unlock()
			log.Println("SMPP connection status:", status.Status())
		}
	}()
	return
}

func (sms *SMS) Close() error {
	return sms.tx.Close()
}

func (sms *SMS) Status() smpp.ConnStatus {
	sms.mu.RLock()
	defer sms.mu.RUnlock()
	return sms.status
}

func (sms *SMS) handler(p pdu.Body) {
	switch p.Header().ID {
	case pdu.DeliverSMID:
		f := p.Fields()
		src := f[pdufield.SourceAddr].String()
		dst := f[pdufield.DestinationAddr].String()
		txt := f[pdufield.ShortMessage].String()
		coding := f[pdufield.DataCoding]
		log.Printf("Incoming SMS [from %q to %q] (%v): %q", src, dst, coding, txt)
		// pretty.Println(p)
	}
}

func (s *SMS) Send(from, to, msg string) (msgID string, err error) {
	sm, err := s.tx.Submit(&smpp.ShortMessage{
		Src:      from,
		Dst:      to,
		Text:     pdutext.Latin1(msg),
		Register: smpp.FinalDeliveryReceipt,
	})
	if err == nil {
		msgID = sm.RespID()
	}
	return
}
