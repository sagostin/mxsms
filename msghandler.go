package main

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/mxsms2/csta"
)

// MessageHandle описывает обработчик входящих сообщений.
type MessageHandle struct {
	*SMSGate                // шаблоны сообщений
	*MX                     // правила разбора телефонных номеров
	phoneRE  *regexp.Regexp // регулярное выражение для разбора сообщения
	client   *csta.Client   // клиент соединения с MX
}

// NewMessageHandler инициализирует и возвращает новый обработчик входящих сообщений.
func NewMessageHandler(sms *SMSGate, mx *MX) *MessageHandle {
	min := 11 - len(mx.Prefix) // по умолчанию будет минимальный телефон без префикса
	if min < 7 {
		min = 11 // если префикс слишком уж длинный, то не учитываем его
	}
	if mx.Short >= 3 && mx.Short <= 6 {
		min = mx.Short
	}
	re := fmt.Sprintf("(?s)\\A\\+?(\\d{%d,11})\\s+(.+)", min)
	// инициализируем обработчик сообщений
	handler := &MessageHandle{
		SMSGate: sms,
		MX:      mx,
		phoneRE: regexp.MustCompile(re),
	}
	return handler
}

// Register возвращает информацию для регистрации обработчика входящих сообщений.
func (mh *MessageHandle) Register(client *csta.Client) csta.EventMap {
	mh.client = client
	return csta.EventMap{
		"message": reflect.TypeOf(incommingMessage{}),
	}
}

// Handle вызывается для обработки разобранных данных входящего сообщения.
func (mh *MessageHandle) Handle(eventData interface{}) (err error) {
	data, ok := eventData.(*incommingMessage)
	if !ok {
		return // поддерживаем только один тип данных для обработки
	}
	// отправляем подтверждение получения сообщения
	if err = mh.client.Send(messageAck{data.From, data.MsgID, data.ReqID}); err != nil {
		return
	}
	logEntry := mh.MX.Logger.WithFields(logrus.Fields{
		"id":   data.MsgID,
		"jid":  data.From,
		"name": data.Name,
	})
	// разбираем сообщение и проверяем, что оно начинается на телефонный номер
	submatch := mh.phoneRE.FindStringSubmatch(data.Body)
	if submatch == nil { // телефонный номер не найден
		logEntry.Info("SMS send ignore: no phone")
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.NoPhone))
	}
	phone := submatch[1] // телефонный номер найден в сообщении
	// анализируем длинну телефонного номера и приводим номер к стандарту
	switch l := len(phone); {
	case l >= 3 && l <= 6 && l == mh.Short: // это короткий номер - оставляем как есть
	case l >= 7 && l == 11-len(mh.Prefix): // не полный номер - без префикса
		phone = fmt.Sprintf("%s%s", mh.Prefix, phone)
	case l == 11 && phone[1] != '0': // полный номер телефона
		phone = fmt.Sprintf("%s", phone)
	default: // непонятная длинна телефонного номера или неверный номер
		logEntry.WithField("phone", phone).Info("SMS send ignore bad phone")
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Incorrect, phone))
	}
	logEntry = logEntry.WithField("phone", phone)
	// теперь займемся текстом сообщения: отправляем SMS-сообщение
	err = mh.SMSGate.Send(mh.name, data.From, data.MsgID, phone, submatch[2])
	if err != nil { // сообщение не отправлено
		logEntry.WithError(err).Info("SMS send error")
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Error, err.Error()))
	}
	logEntry.Info("SMS send to phone") // сообщение успешно отправлено
	if err = mh.client.Send(mh.getMessage(data.From, mh.Responses.Accepted, phone)); err != nil {
		return
	}
	return
}

// incommingMessage описывает входящее сообщение.
type incommingMessage struct {
	From  string `xml:"from,attr"`  // уникальный идентификатор пользователя отправившего сообщение
	Name  string `xml:"name,attr"`  // имя отправителя
	MsgID int64  `xml:"msgId,attr"` // уникальный идентификатор сообщения, который использовался при передаче, в формате десятичного числа
	ReqID int64  `xml:"reqId,attr"` // уникальный идентификатор группы, в случае если сообщение передано группе пользователей
	Body  string `xml:",chardata"`  // текст сообщения
}

// messageAck описывает структуру ответа на сообщение.
type messageAck struct {
	From string `xml:"from,attr"`
	UID  int64  `xml:"msgId,attr"`
	GID  int64  `xml:"reqId,attr"`
}
