package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"mxsms/sms"
	"mxsms/zabbix"
)

// SMSTemplates describes message templates.
type SMSTemplates struct {
	NoPhone   string `yaml:"noPhone,omitempty" json:"noPhone,omitempty"` // doesn't start with a phone number
	Incorrect string `yaml:",omitempty" json:"incorrect,omitempty"`      // contains incorrect number
	Accepted  string `yaml:",omitempty" json:"accepted,omitempty"`       // sent
	Delivered string `yaml:",omitempty" json:"delivered,omitempty"`      // delivered
	Error     string `yaml:",omitempty" json:"error,omitempty"`          // sending or delivery error
	Incoming  string `yaml:",omitempty" json:"incoming,omitempty"`       // incoming
}

type SMSCarrier struct {
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

// SMSGate describes the configuration for sending SMS.
type SMSGate struct {
	SMPP      *sms.SMPP    // SMPP connection
	Carriers  []SMSCarrier `json:"carriers"`
	Responses SMSTemplates `yaml:"messageTemplates" json:"responses"` // list of response templates
	MYSQL     string       `yaml:"mySqlLog" json:"mysql,omitempty"`   // initialization of connection to the log
	Zabbix    *zabbix.Log  `yaml:"zabbix" json:"zabbix,omitempty"`
	counter   uint32       // counter of sent messages
	history   History      // history of sent messages
}

func (s *SMSGate) Connect() {
	s.SMPP.Connect() // establish connection with SMPP servers
	go func() {
		for msg := range s.SMPP.Receive {
			// s.Logger.Debugln("Received:", msg)
			switch msg := msg.(type) {
			case sms.Received: // incoming SMS
				s.Receive(msg) // process incoming message
			}
		}
	}()
}

func (s *SMSGate) Close() {
	s.SMPP.Close() // stop connection with SMPP
}

func (s *SMSGate) Send(mxName, jid string, msgID int64, to, msg string) (err error) {
	from := s.history.GetFrom(config.MX[mxName].From, to, jid) // get the best outgoing number
	if from == "" {
		return errors.New("from phone is empty")
	}
	if to == "" {
		return errors.New("to phone is empty")
	}
	phoneType := int64(11 - len(from))
	smsMessage := &sms.SendMessage{From: from, To: to, Text: msg}
	if err = s.SMPP.Send(smsMessage); err != nil { // send SMS
		//zabbixLog.Send("gw.smsc.error", err.Error())
		sglogDB.Insert(mxName, from, to, msg, false, phoneType, msgID, 0)
		return err
	}
	sglogDB.Insert(mxName, from, to, msg, false, phoneType, msgID, 1)
	s.history.Add(mxName, jid, from, to) // add information about phone connection to history
	return nil
}

// Receive processes incoming messages
func (s *SMSGate) Receive(msg sms.Received) {
	incoming := s.Responses.Incoming
	if incoming == "" {
		incoming = "%s: %s"
	}
	// remove plus signs
	if msg.From[0] == '+' {
		msg.From = msg.From[1:]
	}
	if msg.To[0] == '+' {
		msg.To = msg.To[1:]
	}
	phoneType := int64(11 - len(msg.From))
	mxName, jid := s.history.Get(msg.To, msg.From)
	sglogDB.Insert(mxName, msg.From, msg.To, msg.Text, true, phoneType, 0, 2)
	if mxName == "" || jid == "" { // no suitable user found in history to whom this is addressed
		for name, mx := range config.MX { // iterate through all MX server settings
			for from, _ := range mx.From { // iterate through all their phones
				if msg.To == from { // found a matching number
					mxName = name
					jid = mx.DefaultJID
					if jid == "" { // delivery of unrecognized messages is not configured
						return
					}
					goto next // found a suitable one
				}
			}
		}
		// the number to which the message came is not specified for any server
		llog.WithFields(logrus.Fields{
			"from": msg.From,
			"to":   msg.To,
		}).Warnf("SPAM detected: %q", msg.Text)
		return
	}
next:
	mx := config.MX[mxName]
	if mx == nil {
		return
	}
	for mx.handler == nil {
		llog.Debug("MX Handler not initialised... Waiting...")
		time.Sleep(time.Second)
	}
	mx.client.Send(mx.handler.getMessage(
		jid, incoming, msg.From, msg.Text))
	mx.Logger.WithFields(logrus.Fields{
		"jid":  jid,
		"from": msg.From,
		"to":   msg.To,
	}).Info("SMS incoming")
	// check for spam
	for _, from := range mx.From {
		if msg.To == from {
			return // this is not spam - sent to us
		}
	}
	return
}

// getMessage returns a formed command to send a confirmation message
// based on the template text. If the template text is empty, the message is not sent
func (s *SMSGate) getMessage(to, tmpl string, items ...interface{}) *sendMessage {
	if tmpl == "" || to == "" {
		return nil // subject or recipient address is not defined
	}
	return &sendMessage{
		To:    to,
		MsgID: atomic.AddUint32(&s.counter, 1),
		Body:  fmt.Sprintf(tmpl, items...),
	}
}

// sendMessage describes the structure of an outgoing message.
type sendMessage struct {
	XMLName xml.Name `xml:"message"`
	To      string   `xml:"to,attr"`            // unique recipient identifier
	MsgID   uint32   `xml:"msgId,attr"`         // message identifier
	Ext     string   `xml:"ext,attr,omitempty"` // recipient's internal number
	Body    string   `xml:",chardata"`          // message text
}
