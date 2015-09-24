package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	appName        = "MXSMS"                          // название приложения
	version        = "0.5.0"                          // версия
	date           = "2015-09-24"                     // дата сборки
	build          = ""                               // номер сборки в git-репозитории
	detailedLog    = false                            // вывод детальной информации в лог
	logFlags       = log.LstdFlags                    // флаги для вывода в лог по умолчанию
	logOutput      = os.Stderr                        // вывод для лога
	logger         = log.New(logOutput, "", logFlags) // инициализируем вывод в лог
	configFileName = "config.yaml"                    // имя конфигурационного файла
	serverAddr     = ":8080"                          // адрес веб-сервера
	incomingURL    = "/incoming"                      // адрес обработчика входящих SMS
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

	flag.StringVar(&serverAddr, "http", serverAddr, "HTTP-server `port`")
	flag.StringVar(&incomingURL, "sms", incomingURL, "`URL` for incoming SMS")
	flag.StringVar(&configFileName, "config", configFileName, "configuration `fileName`")
	flag.BoolVar(&detailedLog, "debug", detailedLog, "log output full messages")
	flag.Parse() // разбираем параметры запуска приложения
}

func main() {
	// запускаем веб-сервер
	go func() {
		http.HandleFunc(incomingURL, incoming) // обработка входящих СМС
		log.Println("Starting HTTP Server", serverAddr)
		log.Println("Incoming SMS handler at", incomingURL)
		log.Println(http.ListenAndServe(serverAddr, nil))
	}()

	for { // бесконечный цикл загрузки и остановки сервисов
		logger.Printf("Loading %q...", configFileName)
		var err error
		config, err = LoadConfig(configFileName) // загружаем конфигурационный файл
		if err != nil {
			logger.Fatalln("Error loading config:", err)
		}
		config.Start() // запускаем все сервисы
		// инициализируем поддержку сигналов и ждем, когда он случится...
		signal := moitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.Stop() // останавливаем сервисы
		if signal != syscall.SIGUSR1 {
			logger.Println("[THE END]")
			return // заканчиваем нашу работу
		}
		log.Println("Reload signal...")
	}
}

// moitorSignals запускает мониторинг сигналов и возвращает значение, когда получает сигнал.
// В качестве параметров передается список сигналов, которые нужно отслеживать.
func moitorSignals(signals ...os.Signal) os.Signal {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)
	return <-signalChan
}

func incoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allowed", "POST")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if config == nil {
		http.Error(w, "Incoming SMS Service not configured", http.StatusInternalServerError)
		log.Println("Incoming SMS Service not configured")
		return
	}
	msg, err := config.SMSGate.IncomingHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Incoming SMS Error:", err)
		return
	}
	fmt.Printf("SMS: %#v\n", msg)
	w.WriteHeader(http.StatusOK)
}
