package main

import (
	"log"
	"net/http"
	"time"

	"github.com/mdigger/mxsms2/sinch"
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
	Sinch     *sinch.SMS
	From      string          `yaml:",omitempty"` // номер телефона, с которого отправляются SMS
	Check     time.Duration   `yaml:",omitempty"` // задержка перед проверкой статуса
	Responses SMSTemplates    // список шаблонов ответов
	Default   DefaultDelivery `yaml:",omitempty"`
	MaxLength int             `yaml:",omitempty"` // максимальная длинна сообщения

	History // история отправленных сообщений
}

func (s *SMSGate) Send(serviceName, jid string, msgID int64, to, msg string) (smsMsgID int, err error) {
	from := s.From
	smsMsgID, err = s.Sinch.Send(from, to, msg)
	// smsMsgID, err = 15, nil
	if err != nil {
		return
	}
	s.History.Add(serviceName, jid, msgID, from, to, smsMsgID)
	// делаем проверку доставки сообщения
	if s.Check > 0 {
		go func() {
			for {
				time.Sleep(s.Check)
				status, err := s.Status(smsMsgID)
				if err != nil || config == nil {
					return
				}
				service := config.Services[serviceName]
				if service == nil || service.Disabled {
					return
				}
				switch status {
				case "Pending":
					continue // ждем еще
				case "Successful":
					service.client.Send(service.handler.getMessage(
						jid, s.Responses.Delivered, to))
				default:
					fallthrough
				case "Unknown", "Faulted":
					service.client.Send(service.handler.getMessage(
						jid, s.Responses.Error, to+" - "+status))
				}
				return
			}
		}()
	}
	return
}

func (s *SMSGate) Status(msgID int) (status string, err error) {
	return s.Sinch.Status(msgID)
}

func (s *SMSGate) IncomingHTTP(req *http.Request) (msg *sinch.IncomingSMS, err error) {
	msg, err = s.Sinch.IncomingHTTP(req)
	if err != nil {
		log.Println("Error incoming:", err)
		return
	}
	incoming := s.Responses.Incoming
	if incoming == "" {
		incoming = "%s: %s"
	}
	from := msg.From.Endpoint
	if len(from) == 11 {
		from = "+" + from
	}
	item := s.History.Get(from)
	if item != nil {
		service := config.Services[item.Service]
		if service == nil {
			return
		}
		service.client.Send(service.handler.getMessage(
			item.JID, incoming, msg.From.Endpoint, msg.Message))
		return
	}
	if s.Default.Service != "" && s.Default.JID != "" {
		service := config.Services[s.Default.Service]
		if service == nil {
			return
		}
		service.client.Send(service.handler.getMessage(
			s.Default.JID, incoming, msg.From.Endpoint, msg.Message))
	}
	return
}
