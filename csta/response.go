package csta

import "encoding/xml"

// Response описывает ответ или событие, принимаемые от сервера MX.
type Response struct {
	Name string // название события
	ID   uint16 // идентификатор команды
	data []byte // не разобранное содержимое ответа
}

// String возвращает название события.
func (r *Response) String() string { return r.Name }

// Decode декодирует сообщение в указанный объект.
func (r *Response) Decode(v interface{}) error {
	return xml.Unmarshal(r.data, v)
}
