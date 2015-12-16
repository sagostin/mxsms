package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/mdigger/mxsms2/csta"
	"gopkg.in/yaml.v2"
)

// Config описывает конфигурацию сервиса.
type Config struct {
	Services   map[string]*Service // список серверов
	ErrorDelay time.Duration       `yaml:",omitempty"` // время задержки между повторным подключением к серверу
	*SMSGate                       // информация для инициализации SMS
}

// ParseConfig разбирает конфигурацию.
func ParseConfig(data []byte) (config *Config, err error) {
	config = new(Config)
	err = yaml.Unmarshal(data, config)
	return
}

// LoadConfig загружает и разбирает конфигурацию из файла.
func LoadConfig(filename string) (config *Config, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	return ParseConfig(data)
}

// Start запускает соединение со всеми серверами, указанными в конфигурации.
func (c *Config) Start() error {
	// isMoreServices := len(c.Services) > 1 // флаг, что определено несколько сервисов
	c.SMPP.logger = log.New(logOutput, fmt.Sprintf("%-16s ", "SMPP"), logFlags)
	// устанавливаем соединение с SMPP сервером
	go func() {
		for {
			c.SMPP.logger.Printf("Connecting to %q...", c.SMPP.Address)
			if err := c.SMPP.Connect(); err != nil {
				c.SMPP.logger.Printf("Connection error: %v", err)
			}
			if c.SMPP.closing {
				return
			}
			time.Sleep(c.ErrorDelay) // небольшая задержка перед повторным соединением
		}
	}()
	// перебираем все сервисы, определенные в конфигурации
	for name, s := range c.Services {
		s.mu.Lock()
		// обрезаем имя сервиса для вывода в лог до 8 символов
		var logName string
		// if isMoreServices { // если сервисов больше одного, то добавляем в лог имя сервиса
		if utf8.RuneCountInString(name) > 16 {
			// обрезаем до 16 знаков и добавляем многоточие, если имя слишком длинное
			logName = fmt.Sprintf("%s… ", string([]rune(name)[:15]))
		} else {
			// добиваем имя до 16 символов
			logName = fmt.Sprintf("%-16s ", name)
		}
		// } else {
		// 	logName = "" // не задаем имя в логе, если только один сервер
		// }
		// инициализируем вывод в лог
		s.logger = log.New(logOutput, logName, logFlags)
		s.name = name // сохраняем имя сервиса
		s.mu.Unlock()
		// игнорируем закрытые сервисы
		if s.Disabled {
			s.logger.Println("Ignore the disabled service")
			continue // переходим к следующему сервису в списке
		}
		// запускаем сервис асинхронно и обеспечиваем переподключение в случае ошибок
		go func(s *Service) {
			for { // перезапускаем сервис авторматически в случае ошибок соединения
				err := s.Start(c) // запускаем сервис
				if err != nil {
					s.logger.Println("Service start error:", err) // выводим информацию об ошибке
				}
				// проверяем тип ошибки
				switch err := err.(type) {
				case *csta.ErrorCode, *csta.LoginResponce, syscall.Errno:
					s.logger.Println("Service stoped!")
					return // это критические ошибки авторизации
				case *net.OpError:
					if err.Temporary() || err.Timeout() {
						break // не критические ошибки - переустанавливаем соединение
					}
					s.logger.Println("Service stoped!")
					return // это критические ошибки авторизации
				case nil:
					return // плановая остановка сервиса
				}
				// сюда мы попадаем, если ошибка не критична и нужно переустановить соединение
				time.Sleep(c.ErrorDelay) // небольшая задержка перед повторным соединением
			}
		}(s) // изолируем сервис в качестве парамтера, иначе будет запущен только последний
	}
	return nil
}

// Stop останавливает все запущенные сервисы.
func (c *Config) Stop() {
	c.SMPP.Close() // останавливаем соединение с SMPP сервером
	// перебираем все сервисы, определенные в конфигурации
	for _, s := range c.Services {
		if !s.Disabled {
			s.Stop() // останавливаем сервис, если он активен
		}
	}
}
