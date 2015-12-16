package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kr/pretty"
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
	Address  string         // адрес и порт SMPP сервера
	User     string         // логин для авторизации
	Password string         // пароль для авторизации
	Encoding string         // кодировка исходящих сообщений
	Incoming chan<- Message // чтение входящих сообщений
	trx      *smpp.Transceiver
	logger   *log.Logger
	closing  bool
	mu       sync.RWMutex
}

// Connect устанавливает соединение с сервером, авторизуется и начинает читать от него сообщения.
// В случае ошибки соединения возвращается ее описание и функция заканчивается. Рекомендуется
// запускать ее в отдельном потоке для мониторинга подключения.
func (s *SMPP) Connect() (err error) {
	s.mu.Lock()
	s.closing = false
	if s.Incoming == nil {
		s.Incoming = make(chan Message, 1000)
	}
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
			fmt.Println("SUBMIT_SM_RESP ID:", pdu.GetField("message_id").String())
		case smpp.DELIVER_SM:
			// received Deliver Sm
			fmt.Println("DELIVER_SM:")
			// Print all fields
			// for _, v := range pdu.MandatoryFieldsList() {
			// 	f := pdu.GetField(v)
			// 	fmt.Println("\t", v, ":", f)
			// }
			msg := Message{
				From: pdu.GetField("source_addr").String(),
				To:   pdu.GetField("destination_addr").String(),
				Text: pdu.GetField("destination_addr").String(),
			}
			data_coding := pdu.GetField("destination_addr").Value().(int)
			_ = data_coding
			s.Incoming <- msg
			pretty.Println(msg)
			// Respond back to Deliver SM with Deliver SM Resp
			err := trx.DeliverSmResp(pdu.GetHeader().Sequence, smpp.ESME_ROK)
			if err != nil {
				fmt.Println("DeliverSmResp err:", err)
			}
		case smpp.ENQUIRE_LINK_RESP: // ignore
		default:
			fmt.Println("PDU ID:", pdu.GetHeader().Id)
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
	return s.trx.SubmitSm(from, to, msg, &smpp.Params{
		smpp.DEST_ADDR_TON: 1,
		smpp.DEST_ADDR_NPI: 1,
	})
}
