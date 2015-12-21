package sms

import "time"

// Received описывает доставленное и разобранное СМС-сообщение.
// В него включены только те поля, которые мне были интересны.
type Received struct {
	From string // с какого номера
	To   string // на какой номер
	Text string // текст сообщения (в уже декодированном виде)
	Addr string // идентификатор SMPP-сервера
}

type SendMessage struct {
	MXName string   // название MX-сервера из конфигурации
	JID    string   // уникальный идентификатор пользователя в MX
	From   string   // с какого номера
	To     string   // на какой номер
	Text   string   // текст сообщения (в уже декодированном виде)
	Seq    []uint32 // внутренние номера отправленных сообщений
}

type SendResponse struct {
	ID   string // идентификатор сообщения
	Seq  uint32 // внутренний номер сообщения
	Addr string // идентификатор SMPP-сервера
}

type Status struct {
	ID     string    // идентификатор сообщения
	Sub    int       // количество частей СМС
	Dlvrd  int       // количество доставленных
	Submit time.Time // дата отправки сообщения
	Done   time.Time // дата, когда сообщение достигло своего конечного состояния
	Stat   string    // Статус доставки message_state в строковом виде
	Err    int       // Расширенный статус доставки network_error_code
	Text   string    // Тестовое представление
	Addr   string    // идентификатор SMPP-сервера
}
