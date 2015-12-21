package csta

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"
)

// Client описывает соединение с сервером и работу с ним.
type Client struct {
	conn      net.Conn             // соединение с сервером
	counter   uint32               // счетчик отправленных команд
	keepAlive *time.Timer          // таймер для отправки keep-alive сообщений
	mu        sync.Mutex           // блокировка разделяемого доступа
	Logger    *logrus.Entry        // вывод в лог
	isClosed  bool                 // флаг закрытого соединения
	handlers  map[Handler]EventMap // дополнительные обработчики событий
}

// NewClient возвращает нового инициализированного клиента для работы с MX-сервером, который
// работает поверх установленного соединения.
func NewClient(conn net.Conn) *Client {
	return &Client{conn: conn, Logger: logrus.NewEntry(logrus.StandardLogger())}
}

// AddHandler добавляет обработчики событий, которые будут использоваться при разборе событий,
// получаемых с сервера.
func (c *Client) AddHandler(handlers ...Handler) {
	c.mu.Lock()
	if c.handlers == nil {
		c.handlers = make(map[Handler]EventMap, len(handlers))
	}
	for _, handler := range handlers {
		c.handlers[handler] = handler.Register(c)
	}
	c.mu.Unlock()
}

// Close закрывает соединение с сервером, если оно было открыто.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.isClosed {
		return nil // соединение уже закрыто или еще не было открыто
	}
	c.isClosed = true     // взводим флаг закрытого соединения
	return c.conn.Close() // закрываем соединение с сервером
}

// Send отправляет команду на сервер. В качестве команд может быть передана строка или массив байт,
// которые уже из себя представляют команду в формате XML. Кроме этого, можно передать любой объект,
// который будет приведен к формату XML самостоятельно. Плюс, в качестве параметра может быть
// представлен массив команд в формате Commands: в этом случае все команды из этого списка будут
// отправлены по очереди.
func (c *Client) Send(cmd interface{}) (err error) {
	if c.conn == nil || c.isClosed {
		return io.ErrClosedPipe // соединение не установлено или закрыто
	}
	// преобразуем команду к формату XML
	var dataCmd []byte
	switch data := cmd.(type) {
	case nil: // пустая команда
		return nil // команды нет - нечего отправлять
	case string: // строка с командой
		dataCmd = []byte(data)
	case []byte: // строка в виде массива байт
		dataCmd = data
	case Commands: // список команд
		for _, cmd := range data { // перебираем все команды в списке
			if err = c.Send(cmd); err != nil { // отсылаем команду
				return // прерываемся при ошибке отсылки и возвращаем ее
			}
		}
		return // закончили отправку всех команд
	default:
		if dataCmd, err = xml.Marshal(cmd); err != nil {
			return // возвращаем ошибку сериализации в XML
		}
	}
	// проверяем, что есть что отсылать
	if dataCmd == nil {
		return nil
	}
	buf := bufPool.Get().(*bytes.Buffer)                 // получаем буфер из пула
	buf.Reset()                                          // сбрасываем буфер
	buf.Write([]byte{0, 0})                              // записываем разделитель сообщений
	length := uint16(len(dataCmd) + len(xml.Header) + 8) // вычисляем длину сообщения
	binary.Write(buf, binary.BigEndian, length)          // записываем длину
	counter := atomic.AddUint32(&c.counter, 1)           // увеличиваем счетчик...
	if counter > 9999 {                                  // под счетчик отведено только 4 цифры
		atomic.StoreUint32(&c.counter, 1) // сбрасываем счетчик
		counter = 1
	}
	fmt.Fprintf(buf, "%04d", counter) // ...и добавляем его в сообщение
	buf.WriteString(xml.Header)       // добавляем XML-заголовок
	buf.Write(dataCmd)                // добавляем непосредственно текст команды
	_, err = buf.WriteTo(c.conn)      // отправляем команду на сервер
	bufPool.Put(buf)                  // отдаем обратно в пул по окончанию
	if err != nil {                   // в случае ошибки отправляем ее на обработку
		return // возвращаем ошибку
	}
	if c.keepAlive != nil {
		c.mu.Lock()
		c.keepAlive.Reset(DefaultKeepAliveDuration) // отодвигаем таймер на попозже
		c.mu.Unlock()
	}
	c.Logger.WithFields(logrus.Fields{
		"id":       counter,
		"commands": cmd,
	}).Debug("Send")
	return nil
}

// Reading запускает чтение ответов от сервера и их обработку, а так же поддерживает соединение
// с помощью keep-alive сообщений. В случае корректного закрытия соединения процесс чтения
// завершается и ошибки не возвращается. Процесс чтения ответов от сервера является блокирующим,
// поэтому рекомендуется запускать его в отдельном потоке.
func (c *Client) Reading() (err error) {
	// взводим таймер для отправки keep-alive сообщений
	c.mu.Lock()
	c.keepAlive = time.AfterFunc(DefaultKeepAliveDuration, c.sendKeepAlive)
	c.mu.Unlock()
	// по окночании останавливаем таймер и сбрасываем ошибку, если соединение было закрыто через
	// остановку подключения методом Close().
	defer func() {
		c.mu.Lock()
		c.keepAlive.Stop() // останавливаем таймер по окончании
		c.mu.Unlock()
		if err != nil && c.isClosed {
			err = nil // сбрасываем описание ошибки, если соединение было корректно закрыто
		}
	}()
	header := make([]byte, 8) // заголовок ответа
	for {                     // бесконечный цикл чтения всех сообщений из потока
		// читаем заголовок ответа
		if _, err := io.ReadFull(c.conn, header); err != nil {
			return err // возвращаем ошибку чтения
		}
		// получаем длину сообщения, выделяем под него память и читаем само сообщение
		data := make([]byte, binary.BigEndian.Uint16(header[2:4])-8)
		if _, err := io.ReadFull(c.conn, data); err != nil {
			return err // возвращаем ошибку чтения
		}
		// идентификатор команды от сервера
		id, err := strconv.Atoi(string(header[4:]))
		if err != nil {
			c.Logger.WithError(err).Debug("Ignore message with bad ID")
			continue // пропускаем команду с непонятным номером
		}
		// инициализируем XML-декодер, получаем имя события и данные
		xmlDecoder := xml.NewDecoder(bytes.NewReader(data))
	readingToken:
		offset := xmlDecoder.InputOffset() // сохраняем смещение от начала
		token, err := xmlDecoder.Token()   // читаем название XML-элемента
		if err != nil {
			if err != io.EOF { // выводим в лог
				c.Logger.WithError(err).Debug("Ignore error token")
			}
			continue // игнорируем сообщения с неверным XML - читаем следующее сообщение
		}
		// находим начальный элемент XML, а все остальное пропускаем
		startToken, ok := token.(xml.StartElement)
		if !ok { // если это не корневой XML-элемент, то переходим к следующему
			goto readingToken
		}
		eventName := startToken.Name.Local // получаем название события
		c.Logger.WithFields(logrus.Fields{
			"id":   id,
			"data": string(data[offset:]),
		}).Debug("Receive")
		// обработка внутренних событий
		if eventData := defaultClientEvents.GetDataPointer(eventName); eventData != nil {
			// разбираем сами данные, вернувшиеся в описании события
			if err := xmlDecoder.DecodeElement(eventData, &startToken); err != nil {
				return err
			}
			// обрабатываем разобранные данные
			switch data := eventData.(type) {
			case *ErrorCode: // CSTA Error
				return data // возвращаем как ошибку
			case *LoginResponce: // информация о логине
				if data.Code != 0 {
					return data // возвращаем как ошибку
				}
			}
		}
		// перебираем все обработчики сообщений
		for handler, eventMap := range c.handlers {
			// получаем указатель на структуру данных для разбора события
			eventData := eventMap.GetDataPointer(eventName)
			if eventData == nil {
				continue // пропускаем не поддерживаемые события
			}
			// разбираем сами данные, вернувшиеся в описании события
			if err := xmlDecoder.DecodeElement(eventData, &startToken); err != nil {
				c.Logger.WithError(err).WithField("event", eventName).
					Debug("Ignore decode XML error")
				continue // игнорируем элементы, которые не смогли разобрать
			}
			// передаем разобранное событие для обработки
			if err := handler.Handle(eventData); err != nil {
				return fmt.Errorf("%s: %v", eventName, err)
			}
		}
	}
}

// Login отсылает на сервер команду авторизации.
func (c *Client) Login(login Login) error {
	return c.Send(login.loginRequest())
}

// sendKeepAlive отправляет keep-alive сообщение на сервер, как только срабатывает таймер
func (c *Client) sendKeepAlive() {
	if c.conn == nil || c.isClosed {
		return
	}
	// отправляем заранее подготовленную команду keep-alive на сервер.
	// просто мне не хотелось ее каждый раз кодировать в бинарный вид, поэтому это было сделано
	// только один раз, а теперь отправляется в уже готовом виде.
	c.conn.Write([]byte{0x00, 0x00, 0x00, 0x15, 0x30, 0x30, 0x30, 0x30, 0x3c,
		0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x20, 0x2f, 0x3e})
	if c.keepAlive != nil {
		c.mu.Lock()
		c.keepAlive.Reset(DefaultKeepAliveDuration) // отодвигаем таймер на более позднее время
		c.mu.Unlock()
	}
}

// пул буферов, используемых при формировании команд для сервера.
var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer) // инициализируем и возвращаем новый буфер
	},
}
