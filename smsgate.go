package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/mdigger/mxsms2/sms"
)

// SMSTemplates описывает шаблоны сообщений.
type SMSTemplates struct {
	NoPhone   string `yaml:"noPhone,omitempty"` // не начинается с телефонного номера
	Incorrect string `yaml:",omitempty"`        // содержит некорректный номер
	Accepted  string `yaml:",omitempty"`        // отправлено
	Delivered string `yaml:",omitempty"`        // доставлено
	Error     string `yaml:",omitempty"`        // ошибка отправки или доставки
	Incoming  string `yaml:",omitempty"`        // входящее
}

// DefaultDelivery описывает информацию о доставке непонятных сообщений
type DefaultDelivery struct {
	Service string // название сервиса
	JID     string // уникальный идентификатор пользователя
}

// SMSGate описывает конфигурацию для отправки SMS.
type SMSGate struct {
	SMPP      *sms.SMPP       // SMPP-соединение
	Responses SMSTemplates    `yaml:"messageTemplates"`          // список шаблонов ответов
	Default   DefaultDelivery `yaml:"defaultDelivery,omitempty"` // кому доставлять неизвестные
	counter   uint32          // счетчик отправленных сообщений
	history   History         // история отправленных сообщений
}

func (s *SMSGate) Connect() {
	s.SMPP.Connect() // устанавливаем соединение с SMPP серверами
	go func() {
		for msg := range s.SMPP.Receive {
			// s.Logger.Debugln("Received:", msg)
			switch msg := msg.(type) {
			case sms.Received: // входящая SMS
				s.Receive(msg) // обрабатываем входящее сообщение
			}
		}
	}()
}

func (s *SMSGate) Close() {
	s.SMPP.Close() // останавливаем соединение с SMPP
}

func (s *SMSGate) Send(mxName, jid string, msgID int64, to, msg string) (err error) {
	from := s.history.GetFrom(config.MX[mxName].From, to) // получаем лучший исходящий номер
	if from == "" {
		return errors.New("from phone is empty")
	}
	if to == "" {
		return errors.New("to phone is empty")
	}
	smsMessage := &sms.SendMessage{From: from, To: to, Text: msg}
	if err = s.SMPP.Send(smsMessage); err != nil { // отправляем СМС
		return err
	}
	s.history.Add(mxName, jid, from, to) // добавляем информацию о связи телефонов в историю
	return nil
}

// Receive обрабатывает входящие сообщения
func (s *SMSGate) Receive(msg sms.Received) {
	incoming := s.Responses.Incoming
	if incoming == "" {
		incoming = "%s: %s"
	}
	mxName, jid := s.history.Get(msg.To, msg.From)
	if mxName == "" || jid == "" {
		mxName = s.Default.Service
		jid = s.Default.JID
	}
	if mxName == "" || jid == "" {
		return
	}
	mx := config.MX[mxName]
	if mx == nil {
		return
	}
	for mx.handler == nil {
		llog.Debug("MX Handler not initialised... Waiting...")
		time.Sleep(time.Second)
	}
	mx.client.Send(mx.handler.getMessage(
		jid, incoming, msg.From, msg.Text))
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
