package main

import (
	"flag"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
)

var (
	appName        = "MXSMS"                 // название приложения
	version        = "0.6.6"                 // версия
	date           = "2015-12-21"            // дата сборки
	build          = ""                      // номер сборки в git-репозитории
	configFileName = "config.yaml"           // имя конфигурационного файла
	config         *Config                   // загруженная и разобранная конфигурация
	log            = logrus.StandardLogger() // инициализируем сбор логов
)

const MaxErrors = 10 // максимально допустимое количество ошибок подключения

func main() {
	logrus.SetLevel(logrus.DebugLevel) // уровень отладки
	hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	if err == nil {
		logrus.AddHook(hook)
	}

	flag.StringVar(&configFileName, "config", configFileName, "configuration `fileName`")
	flag.Parse() // разбираем параметры запуска приложения

	// выводим в лог информацию о приложении и версии
	logEntry := log.WithField("version", version)
	if build != "" { // добавляем билд к номеру версии
		logEntry = logEntry.WithField("build", build)
	}
	if date != "" { // добавляем дату сборки к версии
		logEntry = logEntry.WithField("buildDate", date)
	}
	logEntry.Info(appName)

	// запускаем бесконечный цикл чтения конфигурации и установки соединения
	for {
		// загружаем и разбираем конфигурационный файл
		logEntry := log.WithField("filename", configFileName)
		var err error
		config, err = LoadConfig(configFileName) // загружаем конфигурационный файл
		if err != nil {
			logEntry.WithError(err).Fatal("Error loading config")
		}
		logEntry.WithField("mx", len(config.MX)).Info("Config loaded")

		config.MXConnect()       // запускаем асинхронно соединение с MX
		config.SMSGate.Connect() // устанавливаем соединение с SMPP серверами
		// инициализируем поддержку системных сигналов и ждем, когда он случится...
		signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.SMSGate.Close() // останавливаем соединение с SMPP
		config.MXClose()       // останавливаем соединение с MX серверами
		// проверяем, что сигнал не является сигналом перечитать конфиг
		if signal != syscall.SIGUSR1 {
			log.Info("The end")
			return // заканчиваем нашу работу
		}
		log.Info("Reload") // перечитываем конфиг и начинаем все с начала
	}
}

// monitorSignals запускает мониторинг сигналов и возвращает значение, когда получает сигнал.
// В качестве параметров передается список сигналов, которые нужно отслеживать.
func monitorSignals(signals ...os.Signal) os.Signal {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)
	return <-signalChan
}
