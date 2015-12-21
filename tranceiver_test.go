package main

import (
	"os"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/kr/pretty"
	"github.com/mdigger/smpp"
	"github.com/rifflock/lfshook"
	"github.com/x-cray/logrus-prefixed-formatter"
)

func TestScan(t *testing.T) {
	stat := `id:881444543 sub:001 dlvrd:000 submit date:1512201521 done date:1512201521 stat:ACCEPTD err:000 text:ACK/OK      `
	re := regexp.MustCompile(`^\s*id:(\d+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) text:(.+?)\s*$`)
	parts := re.FindStringSubmatch(stat)
	status := Status{
		ID:     parts[1],
		Sub:    0,
		Dlvrd:  0,
		Submit: time.Now(),
		Done:   time.Now(),
		Stat:   parts[6],
		Err:    0,
		Text:   parts[8],
	}
	var err error
	status.Sub, err = strconv.Atoi(parts[2])
	if err != nil {
		t.Error(err)
	}
	status.Dlvrd, err = strconv.Atoi(parts[3])
	if err != nil {
		t.Error(err)
	}
	status.Submit, err = time.Parse(statusTimeFormat, parts[4])
	if err != nil {
		t.Error(err)
	}
	status.Done, err = time.Parse(statusTimeFormat, parts[5])
	if err != nil {
		t.Error(err)
	}
	status.Err, err = strconv.Atoi(parts[7])
	if err != nil {
		t.Error(err)
	}
	pretty.Println(status)
}

var (
	// 	text = `You can paste Unicode messages directly into this field. Don't forget to set data_coding to 8 if using Unicode.
	// You can paste Unicode messages directly into this field. Don't forget to set data_coding to 8 if using Unicode.
	// You can paste Unicode messages directly into this field. Don't forget to set data_coding to 8 if using Unicode.`
	addr = "67.231.1.30:2775"
	from = "14086751455"
	to   = "14154292837"
	to2  = "14086751475"
	// addr = "127.0.0.1:2775"
	// from = "14086751455"
	// to   = "14086751455"
	texts = []string{
		`Test message 1.`,
		`Тестовое сообщение 2`,
		`Test message 3 with Ā`,
		`Тестовое сообщение 4 на русском языке. Оно несколько длиннее, чем умещается в стандартной СМС.`,
	}
)

func TestSMPPTranceiver(t *testing.T) {
	// формируем параметры авторизации
	bindParams := smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   "Zultys",
		smpp.PASSWORD:    "unmQF932",
	}
	logrus.SetLevel(logrus.DebugLevel)
	logrus.AddHook(lfshook.NewHook(lfshook.PathMap{
		logrus.InfoLevel:  "info.log",
		logrus.ErrorLevel: "error.log",
		logrus.DebugLevel: "debug.log",
		logrus.WarnLevel:  "warning.log",
	}))
	logrus.SetFormatter(new(prefixed.TextFormatter))
	Logger := logrus.StandardLogger().WithField("smpp", addr)
	trx, err := NewTransceiver(addr, 0, bindParams, Logger)
	if err != nil {
		Logger.WithError(err).Error("Connect error")
		t.Fatal(err)
	}
	defer trx.Close()
	go trx.reading() // запускаем получение данных с сервера
	for _, text := range texts {
		time.Sleep(time.Second * 2)
		seq, err := trx.Send(from, to2, text)
		if err != nil {
			Logger.WithError(err).Error("Send error")
			t.Error(err)
		}
		Logger.WithField("seq", seq).Info("Sended")
	}
	// инициализируем поддержку системных сигналов и ждем, когда он случится...
	signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
	_ = signal
}
