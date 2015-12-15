package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	appName        = "MXSMS"                          // название приложения
	version        = "0.6.0"                          // версия
	date           = "2015-12-15"                     // дата сборки
	build          = ""                               // номер сборки в git-репозитории
	detailedLog    = false                            // вывод детальной информации в лог
	logFlags       = log.LstdFlags                    // флаги для вывода в лог по умолчанию
	logOutput      = os.Stderr                        // вывод для лога
	logger         = log.New(logOutput, "", logFlags) // инициализируем вывод в лог
	configFileName = "config.yaml"                    // имя конфигурационного файла
)

var config *Config // загруженная и разобранная конфигурация

func init() {
	// выводим версию приложения в лог
	fmt.Fprintf(logOutput, "### %s %s", appName, version)
	if build != "" { // добавляем билд к номеру версии
		fmt.Fprintf(logOutput, " [#%s]", build)
	}
	if date != "" { // добавляем дату сборки к версии
		fmt.Fprintf(logOutput, " (%s)", date)
	}
	fmt.Fprintln(logOutput) // переход на новую строку

	flag.StringVar(&configFileName, "config", configFileName, "configuration `fileName`")
	flag.BoolVar(&detailedLog, "debug", detailedLog, "log output full messages")
	flag.Parse() // разбираем параметры запуска приложения
}

func main() {
	for { // бесконечный цикл загрузки и остановки сервисов
		logger.Printf("Loading %q...", configFileName)
		var err error
		config, err = LoadConfig(configFileName) // загружаем конфигурационный файл
		if err != nil {
			logger.Fatalln("Error loading config:", err)
		}
		config.Start() // запускаем все сервисы
		// инициализируем поддержку сигналов и ждем, когда он случится...
		signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.Stop() // останавливаем сервисы
		if signal != syscall.SIGUSR1 {
			logger.Println("[THE END]")
			return // заканчиваем нашу работу
		}
		log.Println("Reload signal...")
	}
}

// monitorSignals запускает мониторинг сигналов и возвращает значение, когда получает сигнал.
// В качестве параметров передается список сигналов, которые нужно отслеживать.
func monitorSignals(signals ...os.Signal) os.Signal {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)
	return <-signalChan
}
