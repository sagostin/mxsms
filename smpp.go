package main

import (
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
	Address    string        // адрес и порт SMPP сервера
	User       string        // логин для авторизации
	Password   string        // пароль для авторизации
	OnIncoming func(Message) // обработчик входящих сообщений
	trx        *smpp.Transceiver
	logger     *log.Logger
	closing    bool
	mu         sync.RWMutex
}

// Connect устанавливает соединение с сервером, авторизуется и начинает читать от него сообщения.
// В случае ошибки соединения возвращается ее описание и функция заканчивается. Рекомендуется
// запускать ее в отдельном потоке для мониторинга подключения.
func (s *SMPP) Connect() (err error) {
	s.mu.Lock()
	s.closing = false
	s.mu.Unlock()
	trx, err := smpp.NewTransceiver(s.Address, EnquireLinkDuration, smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.User,
		smpp.PASSWORD:    s.Password,
	})
	if err != nil {
		return
	}
	s.trx = trx
	// start reading PDUs
	for {
		pdu, err := trx.Read() // This is blocking
		if err != nil {
			s.mu.RLock()
			if s.closing { // проверяем, что соединения закрывается нами
				s.mu.RUnlock()
				return nil
			}
			s.mu.RUnlock()
			return err
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
}

func (s *SMPP) Close() error {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	return s.trx.Close()
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
	return s.trx.SubmitSm(from, to, msg, &smpp.Params{
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
