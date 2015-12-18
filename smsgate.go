package main

import (
	"encoding/xml"
	"fmt"
	"sync/atomic"
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
	Responses SMSTemplates    // список шаблонов ответов
	Default   DefaultDelivery `yaml:",omitempty"`
	MaxLength int             `yaml:",omitempty"` // максимальная длинна сообщения
	counter   uint32          // счетчик отправленных сообщений
	history   History         // история отправленных сообщений
}

func (s *SMSGate) Send(serviceName, jid string, msgID int64, to, msg string) (seq uint32, err error) {
	from := s.From
	seq, err = s.SMPP.Send(from, to, msg)
	if err != nil {
		return
	}
	s.history.Add(serviceName, jid, msgID, from, to, seq)
	return
}

// Incoming обрабатывает входящие сообщения
func (s *SMSGate) Incoming(msg Message) {
	incoming := s.Responses.Incoming
	if incoming == "" {
		incoming = "%s: %s"
	}
	var (
		serviceName = s.Default.Service
		JID         = s.Default.JID
	)
	if item := s.history.Get(msg.From); item != nil {
		serviceName = item.Service
		JID = item.JID
	}
	if serviceName == "" || JID == "" {
		return
	}
	service := config.Services[serviceName]
	if service == nil {
		return
	}
	service.client.Send(service.handler.getMessage(
		JID, incoming, msg.From, msg.Text))
	return
}

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
