package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/mdigger/mxsms3/csta"
)

// PhoneInfo описывает правила разбора телефонных номеров
type PhoneInfo struct {
	Short  int      `yaml:",omitempty"`              // длина короткого телефонного номера
	Prefix string   `yaml:"defaultPrefix,omitempty"` // префикс неполного телефонного номера
	From   []string // список исходящих телефонных номеров
}

// MX описывает конфигурацию сервиса, включая необходимые данные для подключения к серверу
// и авторизации пользователя.
type MX struct {
	name       string          // имя сервиса
	csta.Addr  `yaml:"server"` // адрес сервера
	csta.Login                 // информация для авторизации
	PhoneInfo  `yaml:"phones"` // информация для разбора телефонных номеров
	Disabled   bool            `yaml:",omitempty"` // флаг игнорируемого сервиса
	Logger     *logrus.Entry   `yaml:"-"`          // лог для вывода информации о сервисе
	client     *csta.Client    // клиент соединения с MX-сервером
}

// Connect устанавливает соединение и запускает сервис.
func (mx *MX) Connect() error {
	if mx.Logger == nil { // инициализируем поддержку лога
		mx.Logger = logrus.NewEntry(logrus.StandardLogger())
	}
	if mx.name != "" { // добавляем имя сервера в лог, если оно определено
		mx.Logger = mx.Logger.WithField("mx", mx.name)
	}
	if mx.Disabled {
		mx.Logger.Warning("Ignore disabled")
		return nil
	}
	conn, err := mx.Addr.Dial()
	if err != nil {
		mx.Logger.WithError(err).Error("Connecting error")
		return err // возвращаем ошибку установки соединения с сервером
	}
	mx.Logger.WithField("host", mx.Addr.FullAddr()).Info("Connected")
	// инициализируем клиента
	client := csta.NewClient(conn)
	defer client.Close()
	client.Logger = mx.Logger
	if err := client.Login(mx.Login); err != nil {
		mx.Logger.WithError(err).Error("Authorizing error")
		return err // ошибка отсылки авторизационной команды на сервер
	}
	mx.Logger.WithField("login", mx.Login.User).Info("Authorized")
	mx.client = client
	// запускаем процесс чтения ответов от сервера
	err = client.Reading()
	if err != nil {
		mx.Logger.WithError(err).Error("MX error")
	}
	return err
}

// Close останавливает запущенный сервис.
func (mx *MX) Close() error {
	if mx.client == nil {
		return nil // клиент подключения к серверу не инициализирован
	}
	mx.Logger.Info("Close")
	return mx.client.Close() // останавливаем соединение с сервером
}
