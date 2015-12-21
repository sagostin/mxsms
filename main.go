package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	appName        = "MXSMS"                 // название приложения
	version        = "0.6.5"                 // версия
	date           = "2015-12-19"            // дата сборки
	build          = ""                      // номер сборки в git-репозитории
	configFileName = "config.yaml"           // имя конфигурационного файла
	config         *Config                   // загруженная и разобранная конфигурация
	log            = logrus.StandardLogger() // инициализируем сбор логов
)

const MaxErrors = 10 // максимально допустимое количество ошибок подключения

func main() {
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
		logEntry.WithField("mx-servers", len(config.MX)).Info("Config loaded")

		// устанавливаем соединение с SMPP-серверами
		config.SMPP.Connect()
		logEntry.WithField("smpp-servers", config.SMPP.Connected()).Info("SMPP Connection")

		// запускаем асинхронно соединение с MX и обеспечиваем переподключение в случае ошибок
		for _, mx := range config.MX {
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
				mx.Logger.Warning("MX connection stoped")
				return // остановка сервиса
			}(mx) // изолируем сервис в качестве парамтера, иначе будет запущен только последний
		}

		// инициализируем поддержку системных сигналов и ждем, когда он случится...
		signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.SMPP.Close()            // закрываем SMPP-соединения
		for _, mx := range config.MX { // останавливаем все запущенные соединения с MX
			mx.Close()
		}
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
