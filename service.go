package main

import (
	"log"
	"sync"

	"github.com/mdigger/mxsms2/csta"
)

// PhoneInfo описывает правила разбора телефонных номеров
type PhoneInfo struct {
	Short  int    `yaml:",omitempty"` // длина короткого телефонного номера
	Prefix string `yaml:",omitempty"` // префикс не полного телефонного номера
}

// Service описывает конфигурацию сервиса, включая необходимые данные для подключения к серверу
// и авторизации пользователя.
type Service struct {
	name       string                    // имя сервиса
	csta.Addr  `yaml:"server"`           // адрес сервера
	csta.Login                           // информация для авторизации
	PhoneInfo  `yaml:"phones,omitempty"` // информация для разбора телефонных номеров
	Disabled   bool                      `yaml:",omitempty"` // флаг игнорируемого сервиса
	client     *csta.Client              // клиент соединения с MX-сервером
	handler    *MessageHandle
	logger     *log.Logger // лог для вывода информации о сервисе
	mu         sync.Mutex
}

// Start устанавливает соединение и запускает сервис.
func (s *Service) Start(c *Config) error {
	// устанавливаем соединение с сервером
	s.logger.Printf("Connecting to %s", s.FullAddr())
	conn, err := s.Addr.Dial()
	if err != nil {
		return err // возвращаем ошибку установки соединения с сервером
	}
	// инициализируем клиента
	client := csta.NewClient(conn)
	defer client.Close()
	client.SetLogger(s.logger, detailedLog)
	// инициализируем обработчик сообщений
	s.handler = NewMessageHandler(c.SMSGate, s)
	client.AddHandler(s.handler)
	// отсылаем авторизационную информацию
	s.logger.Printf("Authorization (%q)", s.User)
	if err := client.Login(s.Login); err != nil {
		return err // ошибка отсылки авторизационной команды на сервер
	}
	// сохраняем ссылку на клиента соединения
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
	// запускаем процесс чтения ответов от сервера
	return client.Reading()
}

// Stop останавливает запущенный сервис.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return nil // клиент подключения к серверу не инициализирован
	}
	s.logger.Println("Stoping service")
	return s.client.Close() // останавливаем соединение с сервером

}
