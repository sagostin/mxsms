package main

import (
	"errors"
	"log"
	"sync"
	"time"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/mdigger/smpp"
)

var EnquireLinkDuration = time.Second * 10

type Message struct {
	From string
	To   string
	Text string
}

// SMPP описывает соединение с сервером SMPP.
type SMPP struct {
	Address    []string            // адрес и порт SMPP сервера
	User       string              // логин для авторизации
	Password   string              // пароль для авторизации
	OnIncoming func(Message)       // обработчик входящих сообщений
	trx        []*smpp.Transceiver // соединеия с сервером
	logger     *log.Logger
	closing    bool
	mu         sync.RWMutex
}

func (s *SMPP) Connect() {
	s.trx = make([]*smpp.Transceiver, len(s.Address))
	for n := range s.Address {
		// устанавливаем соединение с SMPP сервером
		go func(n int) {
			for {
				s.logger.Printf("Connecting to %v", s.Address[n])
				if err := s.connect(n); err != nil {
					s.logger.Printf("Connection error: %v", err)
				}
				if s.closing {
					return
				}
				time.Sleep(time.Second * 5) // небольшая задержка перед повторным соединением
			}
		}(n)
	}
}

// Connect устанавливает соединение с сервером, авторизуется и начинает читать от него сообщения.
// В случае ошибки соединения возвращается ее описание и функция заканчивается. Рекомендуется
// запускать ее в отдельном потоке для мониторинга подключения.
func (s *SMPP) connect(n int) (err error) {
	s.mu.Lock()
	s.closing = false
	s.mu.Unlock()
	trx, err := smpp.NewTransceiver(s.Address[n], EnquireLinkDuration, smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.User,
		smpp.PASSWORD:    s.Password,
	})
	if err != nil {
		return
	}
	s.mu.Lock()
	s.trx[n] = trx
	s.mu.Unlock()
	// start reading PDUs
	for {
		pdu, err := trx.Read() // This is blocking
		if err != nil {
			s.mu.RLock()
			if s.closing { // проверяем, что соединения закрывается нами
				s.mu.RUnlock()
				err = nil
			}
			s.mu.RUnlock()
			break
		}
		// Transceiver auto handles EnquireLinks
		switch pdu.GetHeader().Id {
		case smpp.SUBMIT_SM_RESP:
			// message_id should match this with seq message
			s.logger.Println("SUBMIT_SM_RESP ID:", pdu.GetField(smpp.MESSAGE_ID).String())
		case smpp.DELIVER_SM:
			// received Deliver Sm
			// Print all fields
			// for _, v := range pdu.MandatoryFieldsList() {
			// 	f := pdu.GetField(v)
			// 	fmt.Println("\t", v, ":", f)
			// }
			msg := Message{
				From: pdu.GetField(smpp.SOURCE_ADDR).String(),
				To:   pdu.GetField(smpp.DESTINATION_ADDR).String(),
			}
			txt := pdu.GetField(smpp.SHORT_MESSAGE).ByteArray()
			switch pdu.GetField(smpp.DATA_CODING).Value().(uint8) {
			case 8: // UCS2
				es, _, err := transform.Bytes(unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), txt)
				if err != nil {
					msg.Text = string(txt)
				} else {
					msg.Text = string(es)
				}
			case 2: // latin1 (windows1252)
				es, _, err := transform.Bytes(charmap.Windows1252.NewDecoder(), txt)
				if err != nil {
					msg.Text = string(txt)
				} else {
					msg.Text = string(es)
				}
			default: // raw
				msg.Text = string(txt)
			}
			s.logger.Println("DELIVER_SM:", msg)
			if s.OnIncoming != nil {
				go s.OnIncoming(msg)
			}
			// Respond back to Deliver SM with Deliver SM Resp
			err := trx.DeliverSmResp(pdu.GetHeader().Sequence, smpp.ESME_ROK)
			if err != nil {
				s.logger.Println("DeliverSmResp err:", err)
			}
		case smpp.ENQUIRE_LINK_RESP: // ignore
		default:
			s.logger.Println("PDU ID:", pdu.GetHeader().Id)
		}
	}
	s.mu.Lock()
	s.trx[n] = nil
	s.mu.Unlock()
	return
}

func (s *SMPP) Close() {
	s.mu.Lock()
	s.closing = true
	for _, trx := range s.trx {
		if trx != nil {
			trx.Close()
		}
	}
	s.mu.Unlock()
}

func (s *SMPP) Send(from, to, msg string) (seq uint32, err error) {
	var code int // тип кодировки
	switch isCodepage(msg) {
	case "latin1":
		es, _, err := transform.String(charmap.Windows1252.NewEncoder(), msg)
		if err == nil {
			code = 2
			msg = es
		}
	case "ucs2":
		es, _, err := transform.String(
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder(), msg)
		if err == nil {
			code = 8
			msg = es
		}
	default:
	}
	var trx *smpp.Transceiver
	for _, t := range s.trx {
		if t != nil {
			trx = t
		}
	}
	if trx == nil {
		err = errors.New("smpp: not connected")
		return
	}
	return trx.SubmitSm(from, to, msg, &smpp.Params{
		smpp.DEST_ADDR_TON: 1,
		smpp.DEST_ADDR_NPI: 1,
		smpp.DATA_CODING:   code,
	})
}

func isCodepage(s string) string {
	result := "raw"
	for _, r := range s {
		if r > '\u00FF' {
			return "ucs2"
		}
		if r > '\u007F' {
			result = "latin1"
		}
	}
	return result
}
