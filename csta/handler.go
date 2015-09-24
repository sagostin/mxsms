package csta

import (
	"fmt"
	"reflect"
)

// Handler описывает интерфейс обработчиков событий
type Handler interface {
	Register(*Client) EventMap
	Handle(interface{}) error
}

// EventMap описывает список имен поддерживаемых событий и ассоциированные с ними структуры
// данных для их разбора. Используется при разборе событий, получаемых с сервера.
type EventMap map[string]reflect.Type

// GetDataPointer возвращает указатель на пустую структуру, поддерживающую разбор данных события
// с указанным в параметре именем. Возвращает nil, если данное событие не поддерживается.
func (em EventMap) GetDataPointer(eventName string) interface{} {
	dataType, ok := em[eventName] // получаем структура для разбора события
	if !ok || dataType == nil {
		return nil // событие с таким именем не поддерживается
	}
	// получаем указатель на структуру данных для разбора события
	return reflect.New(dataType).Interface()
}

// defaultClientEvents описывает список поддерживаемых Client событий по умолчанию.
var defaultClientEvents = EventMap{
	"CSTAErrorCode": reflect.TypeOf(ErrorCode{}),
	"loginResponce": reflect.TypeOf(LoginResponce{}),
	"loginFailed":   reflect.TypeOf(LoginResponce{}),
}

// Commands описывает список возвращаемых команд.
type Commands []interface{}

// Add добавляет новую команду к списку.
func (c *Commands) Add(cmd interface{}) {
	if str, ok := cmd.(string); (ok && str == "") || (cmd == nil) {
		return // игнорируем пустые команды
	}
	if c == nil { // инициализируем список, если он был не инициализирован
		*c = make([]interface{}, 0)
	}
	*c = append(*c, cmd) // добавляем команду в список
}

// ErrorCode (CSTAErrorCode) описывает информацию об CSTA-ошибке.
type ErrorCode struct {
	Message string `xml:",any"`
}

// Error возвращает строку с описанием ошибки.
func (e *ErrorCode) Error() string {
	return fmt.Sprintf("CSTA error: %s", e.Message)
}
