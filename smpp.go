package main

import (
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/smpp"
)

// SMPP описывает соединение с сервером SMPP.
type SMPP struct {
	Address         []string            // адрес и порт SMPP сервера
	SystemID        string              `yaml:"systemId"` // логин для авторизации
	Password        string              // пароль для авторизации
	EnquireDuration time.Duration       `yaml:"enquireDuration,omitempty"` // интервал посылки сообщений поддержки соединения
	ReconnectDelay  time.Duration       `yaml:"reconnectDelay,omitempty"`  // время задержки между повторным подключением к серверу
	MaxError        int                 `yaml:"maxError,omitempty"`        // максимально допустимое количество ошибок
	MaxParts        uint8               `yaml:"maxParts,omitempty"`        // максимальное количество разбиений SMS
	Logger          *logrus.Entry       `yaml:"-"`                         // вывод логов
	send            chan SMSSendMessage // канал для отправки SMS
	trxs            []*Transceiver      // список подключенных SMPP-трансиверов
	mu              sync.RWMutex
}

// Connect устанавливает соединение со всеми адресами SMPP, указанными в свойствах.
func (s *SMPP) Connect() {
	s.mu.Lock()
	if s.Logger == nil { // инициализируем поддержку лога
		s.Logger = logrus.NewEntry(logrus.StandardLogger())
	}
	s.send = make(chan SMSSendMessage, 10)       // канал для отправки SMS
	receive := make(chan SMSReceivedMessage, 10) // канал для получения SMS
	status := make(chan SMSStatus, 10)           // канал для получения статусов доставки
	response := make(chan SMSSendResponse, 10)   // канал для получения подтверждений отправки
	go func() {
		for {
			select {
			case msg := <-receive:
				s.Logger.Debugln("Receive:", msg)
			case msg := <-status:
				s.Logger.Debugln("Status:", msg)
			case msg := <-response:
				s.Logger.Debugln("Response:", msg)
			}
		}
	}()
	// формируем параметры авторизации
	bindParams := smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.SystemID,
		smpp.PASSWORD:    s.Password,
	}
	s.trxs = make([]*Transceiver, len(s.Address)) // инициализируем список трансиверов
	s.mu.Unlock()
	// устанавливаем соединение со всеми указанными адресами серверов
	for n, addr := range s.Address {
		go func(addr string) {
			logEntry := s.Logger.WithField("smpp", addr)
			maxErrors := MaxErrors // устанавливаем максимальное количество допустимых ошибок
			if config.SMPP.MaxError > 0 {
				maxErrors = config.SMPP.MaxError
			}
			var lastErrorTime time.Time      // время, когда произошла последняя временная ошибка
			for i := 0; i < maxErrors; i++ { // перезапускаем сервис авторматически в случае ошибок соединения
				trx, err := smpp.NewTransceiver(addr, s.EnquireDuration, bindParams)
				if err != nil {
					logEntry.WithError(err).Error("Connection error")
				}
				switch err := err.(type) { // проверяем тип ошибки
				case *net.OpError:
					if !(err.Temporary() || err.Timeout()) {
						break // это не временная ошибка
					}
					if time.Since(lastErrorTime) > time.Minute*30 {
						i = 0 // сбрасываем счетчик ошибок, если они были давно
					}
					time.Sleep(config.SMPP.ReconnectDelay) // задержка перед следующей попыткой
					lastErrorTime = time.Now()             // запоминаем время ошибки
					continue                               // не критические ошибки - переустанавливаем соединение
				case nil: // соединение успешно установлено
					logEntry.Info("Connected")
					transceiver := &Transceiver{
						addr:        addr,
						Transceiver: trx,
						Logger:      logEntry,
					}
					s.mu.Lock()
					s.trxs[n] = transceiver
					s.mu.Unlock()
					// запускаем обработку сообщений на отправку
					go transceiver.sending(s.send)
					// запускаем получение данных с сервера
					err = transceiver.reading(receive, status, response)
					if err != nil {
						logEntry.WithError(err).Error("SMPP error")
					}
					s.mu.Lock()
					s.trxs[n] = nil // удаляем из списка
					s.mu.Unlock()
					transceiver.Close() // закрываем, если не закрыт
				}
			}
		}(addr)
	}
}

// Connected возвращает количество установленных соединений с SMPP-серверами.
func (s *SMPP) Connected() int {
	var connected int
	s.mu.RLock()
	for _, trx := range s.trxs {
		if trx != nil {
			connected++
		}
	}
	s.mu.RUnlock()
	return connected
}

func (s *SMPP) Close() {
	s.mu.RLock()
	for _, trx := range s.trxs {
		trx.Close()
	}
	s.trxs = nil
	s.mu.RUnlock()
}

// Send отправляет исходящее СМС на обработку и отправку на сервер.
func (s *SMPP) Send(sms SMSSendMessage) {
	s.send <- sms
}
