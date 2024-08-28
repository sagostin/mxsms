package sms

import (
	"errors"
	"sync"
	"time"

	"github.com/mdigger/smpp"
	"github.com/sirupsen/logrus"
	"mxsms/zabbix"
)

const MaxErrors = 10 // maximum allowable number of connection errors

// SMPP describes a connection to the SMPP server.
type SMPP struct {
	Address         []string         // address and port of the SMPP server
	SystemID        string           `yaml:"systemId"` // login for authorization
	Password        string           // password for authorization
	EnquireDuration string           `yaml:"enquireDuration,omitempty"` // interval for sending connection maintenance messages
	ReconnectDelay  string           `yaml:"reconnectDelay,omitempty"`  // delay time between reconnecting to the server
	MaxError        int              `yaml:"maxError,omitempty"`        // maximum allowable number of errors
	MaxParts        uint8            `yaml:"maxParts,omitempty"`        // maximum number of SMS splits
	Logger          *logrus.Entry    `yaml:"-"`                         // log output
	Zabbix          *zabbix.Log      `yaml:"-"`
	Receive         chan interface{} `yaml:"-"` // return channel from transceiver

	send chan *SendMessage       // channel for sending SMS
	trxs map[string]*Transceiver // list of connected SMPP transceivers
	mu   sync.RWMutex
}

// Connect establishes a connection with all SMPP addresses specified in the properties.
func (s *SMPP) Connect() {
	s.mu.Lock()
	if s.Logger == nil { // initialize log support
		s.Logger = logrus.NewEntry(logrus.StandardLogger())
	}
	s.send = make(chan *SendMessage)                       // channel for sending SMS
	s.Receive = make(chan interface{})                     // channel for receiving SMS
	s.trxs = make(map[string]*Transceiver, len(s.Address)) // list of established connections
	if s.MaxParts > 0 {
		MaxParts = int(s.MaxParts) // set the maximum allowable number of SMS parts
	}
	s.mu.Unlock()
	// form authorization parameters
	bindParams := smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.SystemID,
		smpp.PASSWORD:    s.Password,
	}
	// establish a connection with all specified server addresses
	for _, addr := range s.Address {
		go func(addr string) {
			logEntry := s.Logger.WithField("smpp", addr)
			maxErrors := MaxErrors // set the maximum number of allowable errors
			if s.MaxError > 0 {
				maxErrors = s.MaxError
			}
			var lastErrorTime time.Time      // time when the last temporary error occurred
			for i := 0; i < maxErrors; i++ { // restart the service automatically in case of connection errors
				var key string
				if addr == s.Address[0] {
					key = "east.bw.sms.link"
				} else {
					key = "west.bw.sms.link"
				}
				// establish a connection with the SMPP server
				enquireDuration, _ := time.ParseDuration(s.EnquireDuration)
				trx, err := smpp.NewTransceiver(addr, enquireDuration, bindParams)
				if err != nil {
					s.Zabbix.Send(key, "0")
					logEntry.WithError(err).Error("SMPP Connection error")
					if time.Since(lastErrorTime) > time.Minute*30 {
						i = 0 // reset error counter if errors were long ago
					}
					reconnectDelay, _ := time.ParseDuration(s.ReconnectDelay)

					time.Sleep(reconnectDelay) // delay before next attempt
					lastErrorTime = time.Now() // remember error time
					continue                   // repeat once more
				}
				logEntry.Info("SMPP Connected")
				go func() {
					for {
						s.Zabbix.Send(key, "1")
						time.Sleep(time.Minute)
					}
				}()
				transceiver := &Transceiver{
					addr:        addr,
					Transceiver: trx,
					Logger:      logEntry,
				}
				s.mu.Lock()
				s.trxs[addr] = transceiver
				s.mu.Unlock()
				// start processing messages for sending
				go transceiver.sending(s.send)
				// start receiving data from the server
				err = transceiver.reading(s.Receive)
				s.mu.Lock()
				delete(s.trxs, addr) // remove from the list
				s.mu.Unlock()
				transceiver.Close() // close if not closed
				if err != nil {
					logEntry.WithError(err).Error("SMPP error")
				} else {
					break // planned stop
				}
				logEntry.Warning("SMPP Connection stopped")
			}
		}(addr)
	}
}

func (s *SMPP) Close() {
	s.mu.RLock()
	for _, trx := range s.trxs {
		trx.Close()
	}
	s.trxs = nil
	s.mu.RUnlock()
}

// Send sends an outgoing SMS for processing and sending to the server.
func (s *SMPP) Send(sms *SendMessage) error {
	if s.send == nil {
		return errors.New("smpp not initialized")
	}
	s.send <- sms
	return nil
}
