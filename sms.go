package main

import "time"

// SMSReceivedMessage описывает доставленное и разобранное СМС-сообщение.
// В него включены только те поля, которые мне были интересны.
type SMSReceivedMessage struct {
	Addr   string // идентификатор SMPP-сервера
	From   string // с какого номера
	To     string // на какой номер
	Text   string // текст сообщения (в уже декодированном виде)
	Encode uint8  // тип кодирования сообщения
	// для "нарезанных" СМС доступна дополнительная информация о группе:
	GroupID uint8 // идентификатор группы сообщений
	Counter uint8 // порядковый номер сообщения в группе
	Total   uint8 // всего сообщений в группе
}

type SMSSendMessage struct {
	From string // с какого номера
	To   string // на какой номер
	Text string // текст сообщения (в уже декодированном виде)
}

type SMSSendResponse struct {
	Addr string // идентификатор SMPP-сервера
	ID   string // идентификатор сообщения
	Seq  uint32 // внутренний номер сообщения
}

type SMSStatus struct {
	Addr   string    // идентификатор SMPP-сервера
	ID     string    // идентификатор сообщения
	Sub    int       // количество частей СМС
	Dlvrd  int       // количество доставленных
	Submit time.Time // дата отправки сообщения
	Done   time.Time // дата, когда сообщение достигло своего конечного состояния
	Stat   string    // Статус доставки message_state в строковом виде
	Err    int       // Расширенный статус доставки network_error_code
	Text   string    //
}
