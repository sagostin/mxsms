package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/mxsms2/sqlog"
	"github.com/x-cray/logrus-prefixed-formatter"
)

var (
	appName        = "MXSMS"                 // название приложения
	version        = "0.6.6"                 // версия
	date           = "2015-12-21"            // дата сборки
	build          = ""                      // номер сборки в git-репозитории
	configFileName = "config.yaml"           // имя конфигурационного файла
	config         *Config                   // загруженная и разобранная конфигурация
	llog           = logrus.StandardLogger() // инициализируем сбор логов
	sglogDB        *sqlog.DB                 // лог СМС
)

const MaxErrors = 10 // максимально допустимое количество ошибок подключения

func main() {
	var debugLevel = uint(logrus.InfoLevel)
	flag.StringVar(&configFileName, "config", configFileName, "configuration `fileName`")
	flag.UintVar(&debugLevel, "level", debugLevel, "log `level` [0-5]")
	flag.Parse() // разбираем параметры запуска приложения

	logrus.SetLevel(logrus.Level(debugLevel)) // уровень отладки
	// logrus.SetFormatter(new(logrus.JSONFormatter))
	logrus.SetFormatter(new(prefixed.TextFormatter))
	// hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	// if err == nil {
	// 	logrus.AddHook(hook)
	// }

	// выводим в лог информацию о приложении и версии
	logEntry := llog.WithField("version", version)
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
		logEntry := llog.WithField("filename", configFileName)
		var err error
		config, err = LoadConfig(configFileName) // загружаем конфигурационный файл
		if err != nil {
			logEntry.WithError(err).Fatal("Error loading config")
		}
		logEntry.WithField("mx", len(config.MX)).Info("Config loaded")

		sglogDB, err = sqlog.Connect(config.SMSGate.MYSQL)
		if err != nil {
			logEntry.WithError(err).Fatal("Error connecting to MySQL")
		}

		config.MXConnect()       // запускаем асинхронно соединение с MX
		config.SMSGate.Connect() // устанавливаем соединение с SMPP серверами
		// инициализируем поддержку системных сигналов и ждем, когда он случится...
		signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.SMSGate.Close() // останавливаем соединение с SMPP
		config.MXClose()       // останавливаем соединение с MX серверами
		sglogDB.Close()        // закрываем соединение с логом
		// проверяем, что сигнал не является сигналом перечитать конфиг
		if signal != syscall.SIGUSR1 {
			llog.Info("The end")
			return // заканчиваем нашу работу
		}
		llog.Info("Reload") // перечитываем конфиг и начинаем все с начала
	}
}

// monitorSignals запускает мониторинг сигналов и возвращает значение, когда получает сигнал.
// В качестве параметров передается список сигналов, которые нужно отслеживать.
func monitorSignals(signals ...os.Signal) os.Signal {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)
	return <-signalChan
}
