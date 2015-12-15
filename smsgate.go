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
	SMS       *SMS
	From      []string        `yaml:",omitempty"` // номер телефона, с которого отправляются SMS
	Check     time.Duration   `yaml:",omitempty"` // задержка перед проверкой статуса
	Responses SMSTemplates    // список шаблонов ответов
	Default   DefaultDelivery `yaml:",omitempty"`
	MaxLength int             `yaml:",omitempty"` // максимальная длинна сообщения
	counter   uint32          // счетчик отправленных сообщений
	history   History         // история отправленных сообщений
}

func (s *SMSGate) Send(serviceName, jid string, msgID int64, to, msg string) (smsMsgID string, err error) {
	from := s.From[0]
	log.Printf("SMS from %q to %q", from, to)
	smsMsgID, err = s.SMS.Send(from, to, msg)
	if err != nil {
		return
	}
	s.history.Add(serviceName, jid, msgID, from, to, smsMsgID)
	// // делаем проверку доставки сообщения
	// go func() {
	// 	status, err := s.Status(smsMsgID)
	// 	if err != nil || status == "" || config == nil {
	// 		return
	// 	}
	// 	service := config.Services[serviceName]
	// 	if service == nil || service.Disabled {
	// 		return
	// 	}
	// 	switch status {
	// 	case "Successful":
	// 		service.client.Send(service.handler.getMessage(
	// 			jid, s.Responses.Delivered, to))
	// 	default:
	// 		service.client.Send(service.handler.getMessage(
	// 			jid, s.Responses.Error, to+" - "+status))
	// 	}
	// 	return
	// }()
	return
}

// func (s *SMSGate) Status(msgID int) (status string, err error) {
// 	time.Sleep(s.Check)
// 	delay := s.Check
// 	if delay == 0 {
// 		delay = time.Second * 10
// 	}
// 	for {
// 		status, err = s.Sinch.Status(msgID)
// 		if err != nil || status != "Pending" {
// 			return
// 		}
// 		time.Sleep(delay)
// 	}
// }

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
