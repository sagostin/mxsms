package csta

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mdigger/log"
)

var (
	// ConnectionTimeout задает максимальное время ожидания установки соединения
	// с сервером.
	ConnectionTimeout = time.Second * 20
	// ReadTimeout задает время по умолчанию дл ожидания ответа от сервера.
	ReadTimeout = time.Second * 7
	// KeepAliveDuration задает интервал для отправки keep-alive сообщений в
	// случае простоя.
	KeepAliveDuration = time.Minute
)

// Conn описывает соединение с сервером MX.
type Conn struct {
	Info                      // информация о текущем соединении
	logger        *log.Logger // для логирования команд и событий CSTA
	mul           sync.RWMutex
	conn          net.Conn    // сокетное соединение с сервером MX
	counter       uint32      // счетчик отосланных команд
	keepAlive     *time.Timer // таймер для отсылки keep-alive сообщений
	mu            sync.Mutex
	done          chan error    // канал для уведомления о закрытии соединения
	waitResponses sync.Map      // список каналов для обработки ответов
	eventHandlers eventHandlers // зарегистрированные обработчики событий
	monitors      sync.Map      // запущенные мониторы по их идентификаторам
}

// Connect устанавливает соединение с сервером MX и возвращает его.
func Connect(host string) (*Conn, error) {
	// устанавливаем защищенное соединение с сервером MX
	conn, err := tls.DialWithDialer(
		&net.Dialer{
			Timeout:   ConnectionTimeout,
			KeepAlive: KeepAliveDuration,
		},
		"tcp", host,
		&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}
	var mx = &Conn{
		conn: conn,
		done: make(chan error, 1),
	}
	// запускаем отправку keepAlive сообщений
	mx.keepAlive = time.AfterFunc(KeepAliveDuration, mx.sendKeepAlive)
	go mx.reading() // запускаем процесс чтения ответов от сервера
	return mx, nil
}

// Close закрывает соединение с сервером.
func (c *Conn) Close() error {
	c.log(false, 0, []byte("<close/>"))
	return c.conn.Close()
}

// Done возвращает канал для уведомления о закрытии.
func (c *Conn) Done() <-chan error {
	return c.done
}

// send формирует и отправляет команду на сервер. Возвращает номер отосланной
// команды или 0, если она не была отослана.
func (c *Conn) send(cmd interface{}) (uint16, error) {
	// преобразуем данные команды к формату XML
	var xmlData []byte
	switch data := cmd.(type) {
	case string:
		xmlData = []byte(data)
	case []byte:
		xmlData = data
	default:
		var err error
		if xmlData, err = xml.Marshal(cmd); err != nil {
			return 0, err
		}
	}
	// увеличиваем счетчик отправленных команд (не больше 4-х цифр)
	counter := atomic.AddUint32(&c.counter, 1)
	// 9999 зарезервирован для событий в ответах сервера
	if counter > 9998 {
		counter = 1
		atomic.StoreUint32(&c.counter, 1)
	}
	// формируем бинарное представление команды и отправляем ее
	var buf = buffers.Get().(*bytes.Buffer)
	buf.Reset()             // сбрасываем полученный из пула буфер
	buf.Write([]byte{0, 0}) // первые два байта сообщения нули
	// записываем длину сообщения
	binary.Write(buf, binary.BigEndian, uint16(len(xmlData)+8))
	fmt.Fprintf(buf, "%04d", counter) // идентификатор команды
	buf.Write(xmlData)                // содержимое команды
	_, err := buf.WriteTo(c.conn)     // отсылаем команду
	buffers.Put(buf)                  // освобождаем буфер
	c.log(false, uint16(counter), xmlData)
	if err != nil {
		return 0, err
	}
	// откладываем посылку keepAlive
	c.mu.Lock()
	c.keepAlive.Reset(KeepAliveDuration)
	c.mu.Unlock()
	return uint16(counter), nil
}

// buffers используется как пул буферов для формирования новых команд,
// отправляемых на сервер.
var buffers = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

// Send отправляет команду на сервер. Команда может быть представлена в виде
// string, []byte или любого объекта, который может быть преобразован в формат
// xml.
func (c *Conn) Send(cmd interface{}) error {
	_, err := c.send(cmd)
	return err
}

// SendWithResponseTimeout отправляет команду на сервер и ожидает ответ от
// сервера. Команда может быть представлена в виде string, []byte или любого
// объекта, который может быть преобразован в формат xml. Если сервер не
// возвращает ответ на данную команду, то через timeout вернется ошибка
// ErrTimeout.
func (c *Conn) SendWithResponseTimeout(cmd interface{}, timeout time.Duration) (
	*Response, error) {
	// отсылаем команду на сервер и получаем ее идентификатор
	marshal, err := xml.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	println(marshal)

	id, err := c.send(cmd)
	if err != nil {
		return nil, err
	}
	// ожидаем либо ответа на нашу команду, либо истечения времени
	var (
		event *Response                // ожидаемый ответ
		resp  = make(responseChan, 1)  // канал с ответом
		timer = time.NewTimer(timeout) // таймер ожидания
	)
	// сохраняем канал для отдачи ответа в ассоциации с идентификатором
	// отосланной команды
	c.waitResponses.Store(uint16(id), resp)
	// ожидаем ответа или истечения времени ожидания
	select {
	case event = <-resp: // получен ответ от сервера
		// отдельно обрабатываем ответы с описанием ошибки
		if event.Name == "CSTAErrorCode" {
			cstaError := new(CSTAError)
			if err = event.Decode(cstaError); err == nil {
				err = cstaError // подменяем ошибку
			}
			event = nil // сбрасываем данные
		}
	case <-timer.C: // превышено время ожидания ответа от сервера
		err = ErrTimeout
	}
	c.waitResponses.Delete(id) // удаляем из списка ожидания
	close(resp)                // закрываем канал
	timer.Stop()               // останавливаем таймер по окончании
	return event, err
}

// SendWithResponse отправляет команду на сервер и ожидает ответа
// в течение ReadTimeout, после чего, если ответ не получен, возвращает
// ошибку времени ожидания ErrTimeout.
func (c *Conn) SendWithResponse(cmd interface{}) (*Response, error) {
	return c.SendWithResponseTimeout(cmd, ReadTimeout)
}

// reading запускает процесс чтения ответов от сервера. Процесс прекращается
// при ошибке или закрытии соединения
func (c *Conn) reading() error {
	// читаем и разбираем ответы сервера
	var (
		header = make([]byte, 8) // буфер для разбора заголовка ответа
		err    error             // ошибка чтения ответа
	)
	for {
		// читаем заголовок сообщения
		if _, err = io.ReadFull(c.conn, header); err != nil {
			break
		}
		// разбираем номер команды ответа (для событий - 9999)
		var id uint64
		id, err = strconv.ParseUint(string(header[4:]), 10, 16)
		if err != nil {
			break
		}
		// вычисляем длину ответа
		var length = binary.BigEndian.Uint16(header[2:4]) - 8
		// читаем данные с ответом
		var data = make([]byte, length)
		if _, err = io.ReadFull(c.conn, data); err != nil {
			break
		}
		// разбираем xml ответа
		var (
			xmlDecoder = xml.NewDecoder(bytes.NewReader(data))
			token      xml.Token
		)
	readToken:
		var offset = xmlDecoder.InputOffset() // начало корневого элемента xml
		token, err = xmlDecoder.Token()
		if err != nil {
			break
		}
		// пропускаем все до корневого элемента XML
		startToken, ok := token.(xml.StartElement)
		if !ok {
			goto readToken // игнорируем все до корневого элемента XML.
		}
		c.log(true, uint16(id), data[offset:])
		// формируем ответ
		var resp = &Response{
			Name: startToken.Name.Local, // название элемента
			ID:   uint16(id),            // идентификатор команды
			data: data[offset:],         // неразобранные данные с ответом,
		}
		// отдельно обрабатываем ответы на посланные команды
		if id < 9999 {
			// проверяем, что ответом на эту команду мы интересуемся
			if respChan, ok := c.waitResponses.Load(uint16(id)); ok {
				respChan.(responseChan) <- resp
			}
		}
		// отправляем событие всем зарегистрированным обработчикам
		c.eventHandlers.Send(resp)
		// проверяем на принудительное завершение сессии
		if resp.Name == "Logout" {
			var logout = new(ErrLogout)
			if err = resp.Decode(logout); err == nil {
				err = logout
			}
			break // закрываем соединение
		}
	}
	c.eventHandlers.Close() // закрываем все обработчики событий
	c.mu.Lock()
	c.keepAlive.Stop() // останавливаем отправку keepAlive сообщений
	c.mu.Unlock()
	c.done <- err // отправляем ошибку
	close(c.done) // закрываем канал с уведомлением о закрытии
	return err
}

// sendKeepAlive отсылает на сервер keep-alive сообщение для поддержки активного
// соединения и взводит таймер для отправки следующего.
func (c *Conn) sendKeepAlive() {
	// отправляем keepAlive сообщение на сервер
	// чтобы не создавать его каждый раз, а оно не изменяется, оно создано
	// заранее и приведено в бинарный вид команды
	if _, err := c.conn.Write([]byte{0x00, 0x00, 0x00, 0x15, 0x30, 0x30, 0x30,
		0x30, 0x3c, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x20,
		0x2f, 0x3e}); err == nil {
		// взводим таймер отправки следующего keepAlive сообщения
		c.mu.Lock()
		c.keepAlive.Reset(KeepAliveDuration)
		c.mu.Unlock()
		// c.csta(false, 0, []byte("<keepalive/>"))
		// } else {
		// 	c.csta(false, 0, []byte(fmt.Sprintf("<keepalive error=%q/>", err)))
	}
}

// Handler описывает функцию для обработки событий. Если функция возвращает
// ошибку, то дальнейшая обработка событий прекращается.
type Handler = func(*Response) error

// Stop для остановки обработки событий в Handle.
var Stop error = new(emptyError)

// HandleWait вызывает переданную функцию handler для обработки всех событий с
// названиями из списка events. timeout задает максимальное время ожидания
// ответа от сервера. По истечение времени ожидания возвращается ошибка
// ErrTimeout. Если timeout установлен в 0 или отрицательный, то время ожидания
// ответа не ограничено. Для планового завершения обработки можно в качестве
// ошибки вернуть mx.Stop: выполнение прервется, но в ответе ошибкой будет nil.
func (c *Conn) HandleWait(handler Handler, timeout time.Duration,
	events ...string) (err error) {
	if len(events) == 0 {
		return nil // нет событий для отслеживания
	}
	// создаем канал для получения ответов от сервера и регистрируем его для
	// всех заданных имен событий
	var eventChan = make(chan *Response, 1)
	defer close(eventChan)                      // закрываем наш канал по окончании
	c.eventHandlers.Store(eventChan, events...) // регистрируем для событий
	defer c.eventHandlers.Delete(eventChan)     // удаляем обработчики по окончании

	// взводим таймер ожидания ответа
	var timeoutTimer = time.NewTimer(timeout)
	if timeout <= 0 {
		<-timeoutTimer.C // сбрасываем таймер
	}
	for {
		select {
		case resp := <-eventChan: // получили событие от сервера
			// пустой ответ приходит только в случае закрытия соединения
			if resp == nil {
				timeoutTimer.Stop()
				return nil
			}
			// запускаем обработчик события и анализируем ответ с ошибкой
			switch err = handler(resp); err {
			case nil:
				if timeout > 0 { // сдвигаем таймер, если задано время ожидания
					timeoutTimer.Reset(timeout)
				}
				continue // ждем следующего ответа для обработки
			case Stop:
				return nil
			default:
				return err
			}
		case <-timeoutTimer.C:
			return ErrTimeout // ошибка времени ожидания
		}
	}
}

// Handle просто вызывает HandleWait с установленным временем ожидания 0.
func (c *Conn) Handle(handler Handler, events ...string) error {
	return c.HandleWait(handler, 0, events...)
}

// SendAndWaitTimeout отправляет команду и ожидает ответ с заданным именем
// заданное количество времени.
func (c *Conn) SendAndWaitTimeout(cmd interface{}, name string,
	timeout time.Duration) (*Response, error) {
	// отсылаем команду на сервер и получаем ее идентификатор
	id, err := c.send(cmd)
	if err != nil {
		return nil, err
	}
	var event *Response
	// ловим ответ на нашу команду, но не забываем про возможный ответ
	// с ошибкой
	if err := c.HandleWait(func(resp *Response) error {
		// отдельно обрабатываем ответы с описанием ошибки
		if resp.ID == id && resp.Name == "CSTAErrorCode" {
			var cstaError = new(CSTAError)
			if err := resp.Decode(cstaError); err != nil {
				return err
			}
			return cstaError
		}
		// получили нужное нам событие
		event = resp
		if resp.Name == name {
			return Stop
		}
		// игнорируем другие ответы
		return nil
	}, timeout, name, "CSTAErrorCode"); err != nil {
		return nil, err
	}
	return event, nil
}

// SendAndWait отправляет команду и ожидает ответ с заданным именем.
func (c *Conn) SendAndWait(cmd interface{}, name string) (*Response, error) {
	return c.SendAndWaitTimeout(cmd, name, ReadTimeout)
}
