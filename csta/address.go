package csta

import (
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

// Используемые по умолчанию временные интервалы.
var (
	DefaultConnectionTimeout = time.Second * 5  // время ожидания соединения с сервером.
	DefaultKeepAliveDuration = time.Second * 30 // период посылки keep-alive сообщений.
)

// Addr описывает адрес и параметры для подключения к серверу MX по CSTA-протоколу.
type Addr struct {
	Host           string        // адрес сервера
	Port           int           `yaml:",omitempty"`               // порт
	Secure         bool          `yaml:",omitempty"`               // использовать защищенное соединение
	SkipVerify     bool          `yaml:"skipVerify,omitempty"`     // не проверять валидность сертификата
	Timeout        time.Duration `yaml:",omitempty"`               // время ожидания подключения
	ReconnectDelay time.Duration `yaml:"reconnectDelay,omitempty"` // время задержки между повторным подключением к серверу
	MaxError       int           `yaml:"maxError,omitempty"`       // максимально допустимое количество ошибок
}

// FullAddr возвращает полный адрес сервера, включая порт.
func (a *Addr) FullAddr() string {
	port := a.Port // порт сервера
	if port == 0 {
		if a.Secure {
			port = 7778 // порт по умолчанию для защищенного соединения
		} else {
			port = 7777 // порт по умолчанию для не защищенного соединения
		}
	}
	host := a.Host // адрес сервера
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)) // полный адрес сервера, включая порт
}

// Dial устанавливает и возвращает соединение с сервером.
func (a *Addr) Dial() (net.Conn, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = DefaultConnectionTimeout
	}
	dialer := &net.Dialer{ // установщик соединения
		Timeout:   timeout,          // максимальное время ожидания соединения
		KeepAlive: time.Second * 10, // интервал поддержки соединения
	}
	addr := a.FullAddr() // получаем полный адрес, включая порт
	if a.Secure {        // устанавливаем защищенное соединение
		// не проверяем валидность сертификатов, если задано в настройках
		return tls.DialWithDialer(dialer, "tcp", addr,
			&tls.Config{InsecureSkipVerify: a.SkipVerify})
	}
	return dialer.Dial("tcp", addr) // устанавливаем не защищенное соединение
}
