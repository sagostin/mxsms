package main

import (
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/smpp"
)

// Transceiver описывает соединение с SMPP-сервером и позволяет работать с ним.
type Transceiver struct {
	addr              string        // адрес SMPP-сервера
	*smpp.Transceiver               // соединение с сервером
	Logger            *logrus.Entry // вывод логов
}

// NewTransceiver устаналвивает соединение с SMPP-сервером и возвращает его.
func NewTransceiver(addr string, eli time.Duration, bindParams smpp.Params,
	logEntry *logrus.Entry) (*Transceiver, error) {
	trx, err := smpp.NewTransceiver(addr, eli, bindParams)
	if err != nil {
		return nil, err
	}
	return &Transceiver{
		addr:        addr,
		Transceiver: trx,
		Logger:      logEntry,
	}, nil
}

// Close закрывает ранее установленное соединение
func (trx *Transceiver) Close() error {
	if trx.Transceiver == nil {
		return nil
	}
	return trx.Transceiver.Close()
}

// Send отправляет SMS-сообщение на сервер.
func (trx *Transceiver) Send(sms SMSSendMessage) (seq uint32, err error) {
	text := sms.Text
	// определяем кодировку сообщения
	var code int             // номер кодировки
	for _, r := range text { // перебираем текст по символьно
		// пока оставим только юникодную кодировку для всех случаев расширенных символов
		if r > '\u007F' { // используются не ASCII символы
			code = 8
			break
		}
	}
	// переводим текст в нужную кодировку
	var enc transform.Transformer // трансформер текста
	switch code {                 // в зависимости от подходящей кодировки выбираем соответствующий метод кодирования
	case 8: // ucs8
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
		if es, _, err := transform.String(enc, text); err == nil {
			text = es
		}
	}
	// формируем параметры для отправки сообщения
	params := smpp.Params{
		smpp.DEST_ADDR_TON:       1,
		smpp.DEST_ADDR_NPI:       1,
		smpp.DATA_CODING:         code, // кодировка
		smpp.REGISTERED_DELIVERY: 1,    // присылать отчеты о доставке
	}
	// в зависимости от кодировки, проверяем на максимально допустимую длину одного сообщения
	var maxOneMessageLength, maxMultiplyMessageLength int
	switch code {
	case 0: // raw
		maxOneMessageLength = 160
		maxMultiplyMessageLength = 153
	default:
		maxOneMessageLength = 140
		maxMultiplyMessageLength = 134
	}
	logEntry := trx.Logger.WithFields(logrus.Fields{
		"from": sms.From,
		"to":   sms.To,
		"code": code,
	})
	// проверяем, что сообщение умещается в одно
	if len(text) <= maxOneMessageLength {
		logEntry.Info("Send message")
		return trx.Transceiver.SubmitSm(sms.From, sms.To, text, params) // отправляем как есть
	}
	// сообщение необходимо разбить на несколько
	params[smpp.ESM_CLASS] = 0x40 // выставляем специальный тип, что используется склеивание текста
	// считаем количество необходимых частей
	count := (len(text) + maxMultiplyMessageLength - 1) / maxMultiplyMessageLength
	if count > 3 {
		count = 3 // устанавливаем максимальное количество частей
	}
	// формируем "заголовок" UDH-строки СМС
	// в последнем поле хранится счетчик сообщения, в предпоследнем — количество,
	// а передним ним - случайный идентификатор всей группы сообщений
	udh := []byte{0x5, 0x0, 0x3, byte(rand.Intn(0xff) + 1), byte(count), 0x0}
	// перебираем все части и отправляем их на сервер
	for i := 0; i < count; i++ {
		udh[5] = byte(i + 1)                    // добавляем в заголовок порядковый номер
		start := i * maxMultiplyMessageLength   // начала фрагмента текста
		end := start + maxMultiplyMessageLength // окончание фрагмента
		if end > len(text) {
			end = len(text) // мы пытамся получить больше, чем есть на самом деле
		}
		// объединяем заголовок с куском текста
		msg := string(udh) + text[start:end]
		// fmt.Println(">", msg, "[", len(msg), "]")
		logEntry.WithField("type", "multiple").WithFields(logrus.Fields{
			"count": i + 1,
			"total": count,
		}).Info("Send message")
		seq, err = trx.Transceiver.SubmitSm(sms.From, sms.To, msg, params) // отправляем
		if err != nil {
			return seq, err // в случае ошибки возвращаем информацию о ней и прерываемся
		}
	}
	return
}

// sending принимает сообщения из канала и отправляет их на сервер
func (trx *Transceiver) sending(send <-chan SMSSendMessage) {
	for msg := range send {
		_, err := trx.Send(msg)
		if err != nil {
			trx.Logger.WithError(err).Error("Send error")
		}
	}
}

// reStatus описывает формат сообщения со статусом
var reStatus = regexp.MustCompile(`^\s*id:(\d+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) text:(.+?)\s*$`)

const statusTimeFormat = `0601021504` // формат представления даты в ответе со статусом

// reading запускает синхронный процесс чтения данных, получаемых от сервера.
func (trx *Transceiver) reading(receivedChan chan<- SMSReceivedMessage,
	statusChan chan<- SMSStatus, responseChan chan<- SMSSendResponse) error {
	for {
		pdu, err := trx.Read() // Читаем сообщение от сервера
		if err != nil {
			return err
		}
		logEntry := trx.Logger // инициализируем новую запись в лог
		var id string          // уникальный идентификатор сообщения
		if msgid := pdu.GetField(smpp.MESSAGE_ID); msgid != nil {
			id = msgid.String()
			logEntry = logEntry.WithField("id", id)
		}
		if status := pdu.GetHeader().Status; status != smpp.ESME_ROK {
			logEntry.WithError(status).Error("Message status with error")
		}
		switch pdu.GetHeader().Id { // смотрим на тип сообщения
		case smpp.SUBMIT_SM_RESP: // отосланное нами сообщение
			seq := pdu.GetHeader().Sequence // внутренний номер отправленного сообщения
			logEntry.WithField("seq", seq).Info("Send Response")
			responseChan <- SMSSendResponse{
				Addr: trx.addr, // адрес сервера
				ID:   id,       // внешний уникальный идентификатор сообщения
				Seq:  seq,      // внутренний номер сообщения
			}
		case smpp.DELIVER_SM: // входящее сообщение
			var msg SMSReceivedMessage                          // разобранное сообщение
			msg.Addr = trx.addr                                 // адрес сервера
			txt := pdu.GetField(smpp.SHORT_MESSAGE).ByteArray() // получаем сырой текст сообщения
			if classField := pdu.GetField(smpp.ESM_CLASS); classField != nil {
				class := classField.Value().(uint8) // получаем класс сообщения
				logEntry = logEntry.WithField("class", class)
				if class&0x40 > 0 { // это часть "длинного" сообщения
					msg.GroupID = txt[3] // идентификатор группы сообщений
					msg.Total = txt[4]   // общее количество сообщений в группе
					msg.Counter = txt[5] // номер текущего сообщения в группе
					txt = txt[6:]        // оставшийся текст
					logEntry = logEntry.WithFields(logrus.Fields{
						"group": msg.GroupID,
						"count": msg.Counter,
						"total": msg.Total,
					})
				} else if class&0x4 > 0 { // подтверждение доставки
					parts := reStatus.FindStringSubmatch(string(txt))
					status := SMSStatus{
						Addr:   trx.addr,
						ID:     parts[1],
						Sub:    0,
						Dlvrd:  0,
						Submit: time.Now(),
						Done:   time.Now(),
						Stat:   parts[6],
						Err:    0,
						Text:   parts[8],
					}
					status.Sub, _ = strconv.Atoi(parts[2])
					status.Dlvrd, _ = strconv.Atoi(parts[3])
					status.Submit, _ = time.Parse(statusTimeFormat, parts[4])
					status.Done, _ = time.Parse(statusTimeFormat, parts[5])
					status.Err, _ = strconv.Atoi(parts[7])
					logEntry.WithField("id", status.ID).Infoln("Status:", status.Stat)
					statusChan <- status
					goto sendResponse
				}
			}
			msg.From = pdu.GetField(smpp.SOURCE_ADDR).String()
			msg.To = pdu.GetField(smpp.DESTINATION_ADDR).String()
			logEntry = logEntry.WithFields(logrus.Fields{
				"from": msg.From,
				"to":   msg.To,
			})
			msg.Encode = pdu.GetField(smpp.DATA_CODING).Value().(uint8)
			msg.Text = string(txt)
			switch msg.Encode {
			case 8: // UCS2
				es, _, err := transform.Bytes(unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), txt)
				if err == nil {
					msg.Text = string(es)
				}
			case 3: // latin1 (windows1252)
				es, _, err := transform.Bytes(charmap.Windows1252.NewDecoder(), txt)
				if err == nil {
					msg.Text = string(es)
				}
			}
			logEntry.Info("Received")
			receivedChan <- msg
		sendResponse:
			// подтверждаем получение сообщения
			err := trx.DeliverSmResp(pdu.GetHeader().Sequence, smpp.ESME_ROK)
			if err != nil {
				trx.Logger.WithError(err).Error("DeliverSM Response Error")
			}
		case smpp.ENQUIRE_LINK_RESP, smpp.ENQUIRE_LINK: // подтверждение соединения
			continue // игнорируем
		default: // не обработанный тип сообщения
			logEntry.WithField("type", pdu.GetHeader().Id).Warning("Unknown type")
		}
		// Print all fields
		// pretty.Println(pdu)
		// for _, v := range pdu.MandatoryFieldsList() {
		// 	f := pdu.GetField(v)
		// 	fmt.Println("\t", v, ":", f)
		// }
		// for n, v := range pdu.TLVFields() {
		// 	es, _, err := transform.String(unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), v.String())
		// 	if err == nil {
		// 		fmt.Printf("\t%x %s\n", n, es)
		// 	}
		// }
	}
}
