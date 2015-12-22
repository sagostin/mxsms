package sms

import (
	"bytes"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/smpp"
)

var MaxParts = 8 // максимальное количество частей, на которые разрезается длинное сообщение

// Transceiver описывает соединение с SMPP-сервером и позволяет работать с ним.
type Transceiver struct {
	addr              string        // адрес SMPP-сервера
	*smpp.Transceiver               // соединение с сервером
	Logger            *logrus.Entry // вывод логов
	isClosed          bool          // флаг закрытого соединения
	mu                sync.Mutex    // блокировка разделяемого доступа
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
	trx.mu.Lock()
	trx.isClosed = true // взводим флаг закрытого соединения
	trx.mu.Unlock()
	trx.Logger.Info("SMPP Close")
	return trx.Transceiver.Close()
}

// Send отправляет SMS-сообщение на сервер. В ответ возвращается один или несколько
// внутренних номеров отпраленного сообщения.
func (trx *Transceiver) Send(sms *SendMessage) error {
	if trx.Transceiver == nil || trx.isClosed {
		return io.ErrClosedPipe // соединение не установлено или закрыто
	}
	logEntry := trx.Logger.WithFields(logrus.Fields{
		"from": sms.From,
		"to":   sms.To,
	})
	logEntry.Debugf("SMS send text: %q", sms.Text)
	text := sms.Text
	// определяем кодировку сообщения
	var code int             // номер кодировки
	for _, r := range text { // перебираем текст по символьно
		// if r > '\u007F' { // используются не ASCII символы
		// 	code = 3
		// }
		// пока оставим только юникодную кодировку для всех случаев расширенных символов
		if r > '\u007F' { // используются не ASCII символы
			code = 8
			break
		}
	}
	// переводим текст в нужную кодировку
	text = string(Encode(uint8(code), text))
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
	logEntry = logEntry.WithField("code", code)
	// проверяем, что сообщение умещается в одно
	if len(text) <= maxOneMessageLength {
		logEntry.WithField("length", len(text)).Info("SMS send")
		seq, err := trx.Transceiver.SubmitSm(sms.From, sms.To, text, params) // отправляем как есть
		if err == nil {
			sms.Seq = []uint32{seq}
		}
		return err
	}
	// сообщение необходимо разбить на несколько
	params[smpp.ESM_CLASS] = 0x40 // выставляем специальный тип, что используется склеивание текста
	// считаем количество необходимых частей
	count := (len(text) + maxMultiplyMessageLength - 1) / maxMultiplyMessageLength
	if count > MaxParts {
		count = MaxParts // устанавливаем максимальное количество частей
	}
	// формируем "заголовок" UDH-строки СМС
	// в последнем поле хранится счетчик сообщения, в предпоследнем — количество,
	// а передним ним - случайный идентификатор всей группы сообщений
	udh := []byte{0x5, 0x0, 0x3, byte(rand.Intn(0xff) + 1), byte(count), 0x0}
	// перебираем все части и отправляем их на сервер
	sms.Seq = make([]uint32, 0, count) // инициализируем список идентификаторов
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
		logEntry.WithFields(logrus.Fields{
			"count":  i + 1,
			"total":  count,
			"length": len(msg),
		}).Info("SMS send")
		seq, err := trx.Transceiver.SubmitSm(sms.From, sms.To, msg, params) // отправляем
		if err != nil {
			return err // в случае ошибки возвращаем информацию о ней и прерываемся
		}
		sms.Seq = append(sms.Seq, seq)
	}
	return nil
}

// sending принимает сообщения из канала и отправляет их на сервер
func (trx *Transceiver) sending(send <-chan *SendMessage) {
	for msg := range send {
		err := trx.Send(msg)
		if err != nil {
			trx.Logger.WithError(err).Error("Send error")
		}
	}
}

// reStatus описывает формат сообщения со статусом
var reStatus = regexp.MustCompile(`^\s*id:(\d+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) text:(.+?)\s*$`)

const statusTimeFormat = `0601021504` // формат представления даты в ответе со статусом

// reading запускает синхронный процесс чтения данных, получаемых от сервера.
func (trx *Transceiver) reading(receive chan<- interface{}) (err error) {
	// по окночании сбрасываем ошибку, если соединение было закрыто через
	// остановку подключения методом Close().
	defer func() {
		if err != nil && trx.isClosed {
			err = nil // сбрасываем описание ошибки, если соединение было корректно закрыто
		}
	}()
	incomming := make(map[uint8][][]byte) // кеш входящих сообщений
	for {
		pdu, err := trx.Read() // Читаем сообщение от сервера
		if err != nil {
			if !trx.isClosed {
				receive <- err // отдаем ошибку
			}
			return err
		}
		logEntry := trx.Logger // инициализируем новую запись в лог
		var id string          // уникальный идентификатор сообщения
		if msgid := pdu.GetField(smpp.MESSAGE_ID); msgid != nil {
			id = msgid.String()
			logEntry = logEntry.WithField("id", id)
		}
		if status := pdu.GetHeader().Status; status != smpp.ESME_ROK {
			// receive <- status
			logEntry.WithError(status).Error("SMS status with error")
		}
		switch pdu.GetHeader().Id { // смотрим на тип сообщения
		case smpp.SUBMIT_SM_RESP: // отосланное нами сообщение
			seq := pdu.GetHeader().Sequence // внутренний номер отправленного сообщения
			logEntry.WithField("seq", seq).Info("SMS send response")
			receive <- SendResponse{
				Addr: trx.addr, // адрес сервера
				ID:   id,       // внешний уникальный идентификатор сообщения
				Seq:  seq,      // внутренний номер сообщения
			}
		case smpp.DELIVER_SM: // входящее сообщение
			var msg Received    // разобранное сообщение
			msg.Addr = trx.addr // адрес сервера
			msg.From = pdu.GetField(smpp.SOURCE_ADDR).String()
			msg.To = pdu.GetField(smpp.DESTINATION_ADDR).String()
			txt := pdu.GetField(smpp.SHORT_MESSAGE).ByteArray() // получаем сырой текст сообщения
			datacode := pdu.GetField(smpp.DATA_CODING).Value().(uint8)
			logEntry = logEntry.WithFields(logrus.Fields{
				"from":   msg.From,
				"to":     msg.To,
				"length": len(txt),
				"code":   datacode,
			})
			if classField := pdu.GetField(smpp.ESM_CLASS); classField != nil {
				class := classField.Value().(uint8) // получаем класс сообщения
				logEntry = logEntry.WithField("class", class)
				if class&0x4 > 0 { // подтверждение доставки
					parts := reStatus.FindStringSubmatch(string(txt))
					status := Status{
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
					logEntry.WithField("id", status.ID).Infof("SMS status: %q", status.Stat)
					receive <- status
					goto sendResponse
				} else if class&0x40 > 0 { // это часть "длинного" сообщения
					msgs, ok := incomming[txt[3]] // получаем ссылку на кеш
					if !ok {
						msgs = make([][]byte, txt[4])
						incomming[txt[3]] = msgs // сохраняем в кеше
					}
					msgs[txt[5]-1] = txt[6:]
					logEntry = logEntry.WithFields(logrus.Fields{
						"group": txt[3],
						"total": txt[4],
						"count": txt[5],
					})
					if txt[5] != txt[4] { // получили пока не полное сообщение
						logEntry.Info("SMS received (part)")
						goto sendResponse
					}
					delete(incomming, txt[3])        // удаляем из кеша
					txt = bytes.Join(msgs, []byte{}) // объединяем все в единый текст
				}
			}
			msg.Text = Decode(datacode, txt)
			logEntry.Info("SMS received (full)")
			logEntry.Debugf("SMS received text: %q", msg.Text)
			receive <- msg
		sendResponse:
			// подтверждаем получение сообщения
			err := trx.DeliverSmResp(pdu.GetHeader().Sequence, smpp.ESME_ROK)
			if err != nil && !trx.isClosed {
				trx.Logger.WithError(err).Error("SMS DeliverSM Response Error")
				// receive <- err
			}
		case smpp.ENQUIRE_LINK_RESP, smpp.ENQUIRE_LINK: // подтверждение соединения
			continue // игнорируем
		default: // не обработанный тип сообщения
			logEntry.WithField("type", pdu.GetHeader().Id).Warning("SMS unsupported command type")
		}
	}
}
