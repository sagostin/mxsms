package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// SMSTemplates описывает шаблоны сообщений.
type SMSTemplates struct {
	NoPhone   string // сообщение не начинается с телефонного номера
	Incorrect string // сообщение содержит некорректный номер
	Accepted  string
	Delivered string
	Error     string
	Incoming  string
}

type DefaultDelivery struct {
	Service string
	JID     string
}

// SMSGate описывает конфигурацию для отправки SMS.
type SMSGate struct {
	SMPP      *SMPP
	From      string          `yaml:",omitempty"` // номер телефона, с которого отправляются SMS
	Check     time.Duration   `yaml:",omitempty"` // задержка перед проверкой статуса
	Responses SMSTemplates    // список шаблонов ответов
	Default   DefaultDelivery `yaml:",omitempty"`
	MaxLength int             `yaml:",omitempty"` // максимальная длинна сообщения
	counter   uint32          // счетчик отправленных сообщений
	history   History         // история отправленных сообщений
}

func (s *SMSGate) Send(serviceName, jid string, msgID int64, to, msg string) (seq uint32, err error) {
	from := s.From
	log.Printf("SMS from %q to %q", from, to)
	seq, err = s.SMPP.Send(from, to, msg)
	if err != nil {
		return
	}
	s.history.Add(serviceName, jid, msgID, from, to, seq)
	return
}

// func (s *SMSGate) IncomingHTTP(req *http.Request) (msg *sinch.IncomingSMS, err error) {
// 	msg, err = s.Sinch.IncomingHTTP(req)
// 	if err != nil {
// 		log.Println("Error incoming:", err)
// 		return
// 	}
// 	incoming := s.Responses.Incoming
// 	if incoming == "" {
// 		incoming = "%s: %s"
// 	}
// 	from := msg.From.Endpoint
// 	if len(from) == 11 {
// 		from = "+" + from
// 	}
// 	var (
// 		serviceName = s.Default.Service
// 		JID         = s.Default.JID
// 	)
// 	if item := s.history.Get(from); item != nil {
// 		serviceName = item.Service
// 		JID = item.JID
// 	}
// 	if serviceName == "" || JID == "" {
// 		return
// 	}
// 	service := config.Services[serviceName]
// 	if service == nil {
// 		return
// 	}
// 	service.client.Send(service.handler.getMessage(
// 		JID, incoming, msg.From.Endpoint, msg.Message))
// 	return
// }

// getMessage возвращает сформированную команду для отправки подтверждающего сообщения
// на основе текста шаблона. Если текст шаблона пустой, то сообщение не отправляется
func (s *SMSGate) getMessage(to, tmpl string, items ...interface{}) *sendMessage {
	if tmpl == "" || to == "" {
		return nil // тема письма или адрес получателя не определены
	}
	return &sendMessage{
		To:    to,
		MsgID: atomic.AddUint32(&s.counter, 1),
		Body:  fmt.Sprintf(tmpl, items...),
	}
}

// sendMessage описывает структуру исходящего сообщения.
type sendMessage struct {
	XMLName xml.Name `xml:"message"`
	To      string   `xml:"to,attr"`            // уникальный идентификатор получателя
	MsgID   uint32   `xml:"msgId,attr"`         // идентификатор сообщения
	Ext     string   `xml:"ext,attr,omitempty"` // внутренний номер получателя
	Body    string   `xml:",chardata"`          // текст сообщения
}
