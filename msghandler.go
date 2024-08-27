package main

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/sirupsen/logrus"
	"mxsms/csta"
)

// MessageHandle describes the handler for incoming messages.
type MessageHandle struct {
	*SMSGate                // message templates
	*MX                     // rules for parsing phone numbers
	phoneRE  *regexp.Regexp // regular expression for parsing the message
	client   *csta.Client   // client for connection to MX
}

// NewMessageHandler initializes and returns a new handler for incoming messages.
func NewMessageHandler(sms *SMSGate, mx *MX) *MessageHandle {
	min := 11 - len(mx.Prefix) // by default, it will be the minimum phone number without prefix
	if min < 7 {
		min = 11 // if the prefix is too long, we don't consider it
	}
	if mx.Short >= 3 && mx.Short <= 6 {
		min = mx.Short
	}
	re := fmt.Sprintf("(?s)\\A\\+?(\\d{%d,11})\\s+(.+)", min)
	// initialize the message handler
	handler := &MessageHandle{
		SMSGate: sms,
		MX:      mx,
		phoneRE: regexp.MustCompile(re),
	}
	return handler
}

// Register returns information for registering the incoming message handler.
func (mh *MessageHandle) Register(client *csta.Client) csta.EventMap {
	mh.client = client
	return csta.EventMap{
		"message": reflect.TypeOf(incommingMessage{}),
	}
}

// Handle is called to process the parsed data of an incoming message.
func (mh *MessageHandle) Handle(eventData interface{}) (err error) {
	data, ok := eventData.(*incommingMessage)
	if !ok {
		return // we only support one type of data for processing
	}
	// send confirmation of message receipt
	if err = mh.client.Send(messageAck{data.From, data.MsgID, data.ReqID}); err != nil {
		return
	}
	logEntry := mh.MX.Logger.WithFields(logrus.Fields{
		"id":   data.MsgID,
		"jid":  data.From,
		"name": data.Name,
	})
	// parse the message and check if it starts with a phone number
	submatch := mh.phoneRE.FindStringSubmatch(data.Body)
	if submatch == nil { // phone number not found
		logEntry.Info("SMS send ignore: no phone")
		zabbixLog.Send("gw.sms.unknown.destination", "no phone")
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.NoPhone))
	}
	phone := submatch[1] // phone number found in the message
	// analyze the length of the phone number and bring the number to the standard
	switch l := len(phone); {
	case l >= 3 && l <= 6 && l == mh.Short: // this is a short number - leave as is
	case l >= 7 && l == 11-len(mh.Prefix): // not full number - without prefix
		phone = fmt.Sprintf("%s%s", mh.Prefix, phone)
	case l == 11 && phone[1] != '0': // full phone number
		phone = fmt.Sprintf("%s", phone)
	default: // unclear phone number length or invalid number
		logEntry.WithField("phone", phone).Info("SMS send ignore bad phone")
		zabbixLog.Send("gw.sms.unknown.destination", phone)
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Incorrect, phone))
	}
	logEntry = logEntry.WithField("phone", phone)
	// now let's deal with the message text: send SMS message
	err = mh.SMSGate.Send(mh.name, data.From, data.MsgID, phone, submatch[2])
	if err != nil { // message not sent
		logEntry.WithError(err).Info("SMS send error")
		zabbixLog.Send("gw.sms.delivery.error", err.Error())
		return mh.client.Send(mh.getMessage(data.From, mh.Responses.Error, err.Error()))
	}
	logEntry.Info("SMS send to phone") // message successfully sent
	if err = mh.client.Send(mh.getMessage(data.From, mh.Responses.Accepted, phone)); err != nil {
		return
	}
	return
}

// incommingMessage describes an incoming message.
type incommingMessage struct {
	From  string `xml:"from,attr"`  // unique identifier of the user who sent the message
	Name  string `xml:"name,attr"`  // sender's name
	MsgID int64  `xml:"msgId,attr"` // unique message identifier used during transmission, in decimal format
	ReqID int64  `xml:"reqId,attr"` // unique group identifier, in case the message is sent to a group of users
	Body  string `xml:",chardata"`  // message text
}

// messageAck describes the structure of a message response.
type messageAck struct {
	From string `xml:"from,attr"`
	UID  int64  `xml:"msgId,attr"`
	GID  int64  `xml:"reqId,attr"`
}
