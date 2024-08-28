package csta

import (
	"sync"
)

// eventHandlers содержит список зарегистрированных обработчиков, сгруппированых
// по названиям событий.
type eventHandlers struct {
	handlers sync.Map
}

// Store добавляет новый обработчик для указанных событий.
func (ehs *eventHandlers) Store(ch chan<- *Response, events ...string) {
	for _, event := range events {
		list, ok := ehs.handlers.Load(event)
		if !ok {
			list = mapOfHandlerChan{ch: struct{}{}}
		} else {
			list.(mapOfHandlerChan)[ch] = struct{}{}
		}
		ehs.handlers.Store(event, list) // сохраняем обновленный список
	}
}

// Delete удаляет обработчик из списка зарегистрированных.
func (ehs *eventHandlers) Delete(ch chan<- *Response) {
	ehs.handlers.Range(func(event, list interface{}) bool {
		var handlers = list.(mapOfHandlerChan)
		if _, ok := handlers[ch]; ok {
			if len(handlers) < 2 {
				ehs.handlers.Delete(event)
			} else {
				delete(handlers, ch)
				ehs.handlers.Store(event, handlers)
			}
		}
		return true
	})
}

// Send отсылает событие на все зарегистрированные для него обработчики.
func (ehs *eventHandlers) Send(resp *Response) {
	if list, ok := ehs.handlers.Load(resp.Name); ok {
		for handler := range list.(mapOfHandlerChan) {
			handler <- resp
		}
	}
}

// Close посылает всем зарегистрированным обработчикам пустое событие, чтобы
// они могли корректно прекратить обработку.
func (ehs *eventHandlers) Close() {
	// т.к. для нескольких событий может быть зарегистрирован один и тот же
	// обработчик, то сначала собираем коллекцию уникальных обработчиков
	var handlers = make(mapOfHandlerChan)
	ehs.handlers.Range(func(event, list interface{}) bool {
		for handler := range list.(mapOfHandlerChan) {
			handlers[handler] = struct{}{}
		}
		ehs.handlers.Delete(event) // так удаление будет быстрее
		return true
	})
	// и только теперь отсылаем в них пустые ответы, чтобы они могли корректно
	// закрыть обработку
	for handler := range handlers {
		handler <- nil
	}
}

type (
	// mapOfHandlerChan используется в качестве синонима для описания списка
	// каналов для обработки событий.
	mapOfHandlerChan = map[chan<- *Response]struct{}
	// responseChan описывает канал для получения ответов.
	responseChan = chan *Response
)
