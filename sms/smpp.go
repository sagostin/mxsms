package sms

import (
	"errors"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/smpp"
)

const MaxErrors = 10 // максимально допустимое количество ошибок подключения

// SMPP описывает соединение с сервером SMPP.
type SMPP struct {
	Address         []string         // адрес и порт SMPP сервера
	SystemID        string           `yaml:"systemId"` // логин для авторизации
	Password        string           // пароль для авторизации
	EnquireDuration time.Duration    `yaml:"enquireDuration,omitempty"` // интервал посылки сообщений поддержки соединения
	ReconnectDelay  time.Duration    `yaml:"reconnectDelay,omitempty"`  // время задержки между повторным подключением к серверу
	MaxError        int              `yaml:"maxError,omitempty"`        // максимально допустимое количество ошибок
	MaxParts        uint8            `yaml:"maxParts,omitempty"`        // максимальное количество разбиений SMS
	Logger          *logrus.Entry    `yaml:"-"`                         // вывод логов
	Receive         chan interface{} `yaml:"-"`                         // обратный канал от трансивера

	send chan *SendMessage       // канал для отправки SMS
	trxs map[string]*Transceiver // список подключенных SMPP-трансиверов
	mu   sync.RWMutex
}

// Connect устанавливает соединение со всеми адресами SMPP, указанными в свойствах.
func (s *SMPP) Connect() {
	s.mu.Lock()
	if s.Logger == nil { // инициализируем поддержку лога
		s.Logger = logrus.NewEntry(logrus.StandardLogger())
	}
	s.send = make(chan *SendMessage)                       // канал для отправки SMS
	s.Receive = make(chan interface{})                     // канал для получения SMS
	s.trxs = make(map[string]*Transceiver, len(s.Address)) // список установленных соединений
	if s.MaxParts > 0 {
		MaxParts = int(s.MaxParts) // устанавливаем максимально допустимое количество частей SMS
	}
	s.mu.Unlock()
	// формируем параметры авторизации
	bindParams := smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.SystemID,
		smpp.PASSWORD:    s.Password,
	}
	// устанавливаем соединение со всеми указанными адресами серверов
	for _, addr := range s.Address {
		go func(addr string) {
			logEntry := s.Logger.WithField("smpp", addr)
			maxErrors := MaxErrors // устанавливаем максимальное количество допустимых ошибок
			if s.MaxError > 0 {
				maxErrors = s.MaxError
			}
			var lastErrorTime time.Time      // время, когда произошла последняя временная ошибка
			for i := 0; i < maxErrors; i++ { // перезапускаем сервис авторматически в случае ошибок соединения
				// устанавливаем соединение с SMPP-сервером
				trx, err := smpp.NewTransceiver(addr, s.EnquireDuration, bindParams)
				if err != nil {
					logEntry.WithError(err).Error("SMPP Connection error")
					if time.Since(lastErrorTime) > time.Minute*30 {
						i = 0 // сбрасываем счетчик ошибок, если они были давно
					}
					time.Sleep(s.ReconnectDelay) // задержка перед следующей попыткой
					lastErrorTime = time.Now()   // запоминаем время ошибки
					continue                     // повторяем еще раз
				}
				logEntry.Info("SMPP Connected")
				transceiver := &Transceiver{
					addr:        addr,
					Transceiver: trx,
					Logger:      logEntry,
				}
				s.mu.Lock()
				s.trxs[addr] = transceiver
				s.mu.Unlock()
				// запускаем обработку сообщений на отправку
				go transceiver.sending(s.send)
				// запускаем получение данных с сервера
				err = transceiver.reading(s.Receive)
				s.mu.Lock()
				delete(s.trxs, addr) // удаляем из списка
				s.mu.Unlock()
				transceiver.Close() // закрываем, если не закрыт
				if err != nil {
					logEntry.WithError(err).Error("SMPP error")
				} else {
					break // плановая остановка
				}
				logEntry.Warning("SMPP Connection stopped")
			}
		}(addr)
	}
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
func (s *SMPP) Send(sms *SendMessage) error {
	if s.send == nil {
		return errors.New("smpp not initialized")
	}
	s.send <- sms
	return nil
}
