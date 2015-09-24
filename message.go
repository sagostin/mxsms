package main

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"

	"github.com/mdigger/mxsms2/csta"
)

// MessageHandle описывает обработчик входящих сообщений.
type MessageHandle struct {
	*SMSGate                // шаблоны сообщений
	*Service                // правила разбора телефонных номеров
	phoneRE  *regexp.Regexp // регулярное выражение для разбора сообщения
	counter  uint32         // счетчик отправленных сообщений
	client   *csta.Client
	logger   *log.Logger // вывод в лог
}

// NewMessageHandler инициализирует и возвращает новый обработчик входящих сообщений.
func NewMessageHandler(sms *SMSGate, service *Service) *MessageHandle {
	min := 11 - len(service.Prefix) // по умолчанию будет минимальный телефон без префикса
	if min < 7 {
		min = 11 // если префикс слишком уж длинный, то не учитываем его
	}
	if service.Short >= 3 && service.Short <= 6 {
		min = service.Short
	}
	textLength := sms.MaxLength
	if textLength <= 0 {
		textLength = 160
	}
	re := fmt.Sprintf("(?s)\\A\\+?(\\d{%d,11})\\s+(.{1,%d})", min, textLength)
	// инициализируем обработчик сообщений
	handler := &MessageHandle{
		SMSGate: sms,
		Service: service,
		logger:  service.logger,
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
	// разбираем сообщение и проверяем, что оно начинается на телефонный номер
	submatch := mh.phoneRE.FindStringSubmatch(data.Body)
	if submatch == nil { // телефонный номер не найден
		mh.logger.Printf("✗ [%d] Ignore: %s - no phone", data.MsgID)
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.NoPhone, ""))
	}
	// телефонный номер найден в сообщении
	phone := submatch[1]
	// анализируем длинну телефонного номера и приводим номер к стандарту
	switch l := len(phone); {
	case l >= 3 && l <= 6 && l == mh.Short: // это короткий номер - оставляем как есть
	case l >= 7 && l == 11-len(mh.Prefix): // не полный номер - без префикса
		phone = fmt.Sprintf("+%s%s", mh.Prefix, phone)
	case l == 11 && phone[1] != '0': // полный номер телефона
		phone = fmt.Sprintf("+%s", phone)
	default: // непонятная длинна телефонного номера или неверный номер
		mh.logger.Printf("✗ [%d] Ignore phone %q", data.MsgID, phone)
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Incorrect, phone))
	}
	// теперь займемся текстом сообщения
	// отправляем SMS-сообщение и получаем его идентификатор
	msgID, err := mh.Send(mh.name, data.From, data.MsgID, phone, submatch[2])
	if err != nil {
		// сообщение не отправлено
		mh.logger.Printf("✗ [%d] Send SMS to phone %q error: %s", data.MsgID, phone, err)
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Error, err.Error()))
	}
	// сообщение успешно отправлено
	mh.logger.Printf("✓ [%d] Send SMS to phone %q (#%d): %s",
		data.MsgID, phone, msgID, "Accepted")
	if err = mh.client.Send(mh.getMessage(
		data.From, mh.Responses.Accepted, strconv.Itoa(msgID))); err != nil {
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
