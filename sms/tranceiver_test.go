package sms

import (
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mdigger/smpp"
	"github.com/rifflock/lfshook"
	"github.com/x-cray/logrus-prefixed-formatter"
)

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
		// `Test message 1.`,
		// `Тестовое сообщение 2`,
		// `Test message 3 with Ā`,
		`Тестовое сообщение 4 на русском языке. Оно несколько длиннее, чем умещается в стандартной СМС.`,
		`День зимняя сказка с картинки, как жаль, что прогнозы погоды опять предвещают повышение до нуля и чуть выше - снова начнет все таять. Но пока есть шанс насладиться зимней красотой. В такие дни я понимаю за что люблю зиму - за снег, за все удивительные метаморфозы, что происходят на улицах и на деревьях. Как, наверное, странно не знать, что такое снег, что такое мороз и шарф до самых глаз в холодные дни. Я чувствую как деревенеют брюки, как разгораются от холода колени и пальцы, даже спрятанные в карманах, как хочется скорее спрятаться там, где тепло, там, где много шерстяного и обжигающего в чашках.`,
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
	receive := make(chan interface{}, 10)   // канал для получения SMS
	status := make(chan Status, 10)         // канал для получения статусов доставки
	response := make(chan SendResponse, 10) // канал для получения подтверждений отправки
	go func() {
		for {
			select {
			case msg := <-receive:
				Logger.Debugln("Receive:", msg)
			case msg := <-status:
				Logger.Debugln("Status:", msg)
			case msg := <-response:
				Logger.Debugln("Response:", msg)
			}
		}
	}()
	go trx.reading(receive) // запускаем получение данных с сервера
	for _, text := range texts {
		time.Sleep(time.Second * 10)
		sms := &SendMessage{
			From: from,
			To:   to2,
			Text: text,
		}
		err := trx.Send(sms)
		if err != nil {
			Logger.WithError(err).Error("Send error")
			t.Error(err)
		}
		Logger.WithField("seqs", sms.Seq).Info("Sended")
	}

	time.Sleep(time.Minute * 5)
}
