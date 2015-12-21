package main

import (
	"io/ioutil"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	MX      map[string]*MX // сервера MX
	SMSGate *SMSGate       // настройки работы с SMS
}

// ParseConfig разбирает конфигурацию и инициализирует начальные значения.
func ParseConfig(data []byte) (*Config, error) {
	config := new(Config)
	err := yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	for name, mx := range config.MX {
		if mx.Disabled {
			delete(config.MX, name) // сразу удаляем заблокированные сервера MX
			continue
		}
		mx.name = name                                            // сохраняем имя конфигурации сервера
		mx.Logger = logrus.StandardLogger().WithField("mx", name) // назначаем обработчик логов
	}
	return config, nil
}

// LoadConfig загружает и разбирает конфигурацию из файла.
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseConfig(data)
}

func (c *Config) MXConnect() {
	for _, mx := range c.MX {
		go func(mx *MX) {
			maxErrors := MaxErrors // устанавливаем максимальное количество допустимых ошибок
			if mx.Addr.MaxError > 0 {
				maxErrors = mx.Addr.MaxError
			}
			var lastErrorTime time.Time      // время, когда произошла последняя временная ошибка
			for i := 0; i < maxErrors; i++ { // перезапускаем сервис авторматически в случае ошибок соединения
				err := mx.Connect()        // устанавливаем соединение с сервером
				switch err := err.(type) { // проверяем тип ошибки
				// case *csta.ErrorCode, *csta.LoginResponce, syscall.Errno:
				case *net.OpError:
					if !(err.Temporary() || err.Timeout()) {
						break // это не временная ошибка
					}
					if time.Since(lastErrorTime) > time.Minute*30 {
						i = 0 // сбрасываем счетчик ошибок, если они были давно
					}
					time.Sleep(mx.Addr.ReconnectDelay) // задержка перед следующей попыткой
					lastErrorTime = time.Now()         // запоминаем время ошибки
					continue                           // не критические ошибки - переустанавливаем соединение
				case nil:
					return // плановая остановка сервиса
				}
				break
			}
			mx.Logger.Warning("MX connection stopped")
			return // остановка сервиса
		}(mx) // изолируем сервис в качестве парамтера, иначе будет запущен только последний
	}
}

func (c *Config) MXClose() {
	for _, mx := range c.MX { // останавливаем все запущенные соединения с MX
		mx.Close()
	}
}
