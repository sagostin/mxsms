package main

import (
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	MX   map[string]*MX // сервера MX
	SMPP *SMPP          // настройки SMPP-соединения
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
