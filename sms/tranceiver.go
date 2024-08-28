package sms

import (
	"bytes"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"mxsms/smpp"
)

var MaxParts = 8 // maximum number of parts into which a long message is split

// Transceiver describes a connection to the SMPP server and allows working with it.
type Transceiver struct {
	addr              string        // SMPP server address
	*smpp.Transceiver               // connection to the server
	Logger            *logrus.Entry // log output
	isClosed          bool          // flag for closed connection
	mu                sync.Mutex    // lock for shared access
}

// NewTransceiver establishes a connection with the SMPP server and returns it.
func NewTransceiver(addr string, eli time.Duration, bindParams smpp.Params,
	logEntry *logrus.Entry) (*Transceiver, error) {
	trx, err := smpp.NewTransceiver(addr, eli, bindParams)
	if err != nil {
		return nil, err
	}
	return &Transceiver{
		addr:        addr,
		Transceiver: trx,
		Logger:      logEntry,
	}, nil
}

// Close closes a previously established connection
func (trx *Transceiver) Close() error {
	if trx.Transceiver == nil {
		return nil
	}
	trx.mu.Lock()
	trx.isClosed = true // set the flag for closed connection
	trx.mu.Unlock()
	trx.Logger.Info("SMPP Close")
	return trx.Transceiver.Close()
}

// Send sends an SMS message to the server. In response, it returns one or more
// internal numbers of the sent message.
func (trx *Transceiver) Send(sms *SendMessage) error {
	if trx.Transceiver == nil || trx.isClosed {
		return io.ErrClosedPipe // connection is not established or is closed
	}
	logEntry := trx.Logger.WithFields(logrus.Fields{
		"from": sms.From,
		"to":   sms.To,
	})
	logEntry.Debugf("SMS send text: %q", sms.Text)
	text := sms.Text
	// determine the message encoding
	var code int             // encoding number
	for _, r := range text { // iterate through the text character by character
		// if r > '\u007F' { // non-ASCII characters are used
		// 	code = 3
		// }
		// for now, we'll leave only unicode encoding for all cases of extended characters
		if r > '\u007F' { // non-ASCII characters are used
			code = 8
			break
		}
	}
	// convert the text to the required encoding
	text = string(Encode(uint8(code), text))
	// form parameters for sending the message
	params := smpp.Params{
		smpp.DEST_ADDR_TON:       1,
		smpp.DEST_ADDR_NPI:       1,
		smpp.DATA_CODING:         code, // encoding
		smpp.REGISTERED_DELIVERY: 1,    // send delivery reports
	}
	// depending on the encoding, check for the maximum allowable length of a single message
	var maxOneMessageLength, maxMultiplyMessageLength int
	switch code {
	case 0: // raw
		maxOneMessageLength = 160
		maxMultiplyMessageLength = 153
	default:
		maxOneMessageLength = 140
		maxMultiplyMessageLength = 134
	}
	logEntry = logEntry.WithField("code", code)
	// check if the message fits into one
	if len(text) <= maxOneMessageLength {
		logEntry.WithField("length", len(text)).Info("SMS send")
		seq, err := trx.Transceiver.SubmitSm(sms.From, sms.To, text, params) // send as is
		if err == nil {
			sms.Seq = []uint32{seq}
		}
		return err
	}
	// the message needs to be split into several
	params[smpp.ESM_CLASS] = 0x40 // set a special type indicating that text concatenation is used
	// calculate the number of necessary parts
	count := (len(text) + maxMultiplyMessageLength - 1) / maxMultiplyMessageLength
	if count > MaxParts {
		count = MaxParts // set the maximum number of parts
	}
	// form the "header" of the UDH string of the SMS
	// the last field stores the message counter, the penultimate - the quantity,
	// and before it - a random identifier for the entire group of messages
	udh := []byte{0x5, 0x0, 0x3, byte(rand.Intn(0xff) + 1), byte(count), 0x0}
	// iterate through all parts and send them to the server
	sms.Seq = make([]uint32, 0, count) // initialize the list of identifiers
	for i := 0; i < count; i++ {
		udh[5] = byte(i + 1)                    // add the sequence number to the header
		start := i * maxMultiplyMessageLength   // start of the text fragment
		end := start + maxMultiplyMessageLength // end of the fragment
		if end > len(text) {
			end = len(text) // we're trying to get more than actually exists
		}
		// combine the header with a piece of text
		msg := string(udh) + text[start:end]
		// fmt.Println(">", msg, "[", len(msg), "]")
		logEntry.WithFields(logrus.Fields{
			"count":  i + 1,
			"total":  count,
			"length": len(msg),
		}).Info("SMS send")
		seq, err := trx.Transceiver.SubmitSm(sms.From, sms.To, msg, params) // send
		if err != nil {
			return err // in case of an error, return information about it and break
		}
		sms.Seq = append(sms.Seq, seq)
	}
	return nil
}

// sending receives messages from the channel and sends them to the server
func (trx *Transceiver) sending(send <-chan *SendMessage) {
	for msg := range send {
		err := trx.Send(msg)
		if err != nil {
			trx.Logger.WithError(err).Error("Send error")
		}
	}
}

// reStatus describes the format of a status message
var reStatus = regexp.MustCompile(`^\s*id:(\d+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) text:(.+?)\s*$`)

const statusTimeFormat = `0601021504` // format for representing date in the status response

// reading starts a synchronous process of reading data received from the server.
func (trx *Transceiver) reading(receive chan<- interface{}) (err error) {
	// at the end, reset the error if the connection was closed through
	// stopping the connection using the Close() method.
	defer func() {
		if err != nil && trx.isClosed {
			err = nil // reset the error description if the connection was correctly closed
		}
	}()
	incomming := make(map[uint8][][]byte) // cache for incoming messages
	for {
		pdu, err := trx.Read() // Read a message from the server
		if err != nil {
			if !trx.isClosed {
				receive <- err // pass the error
			}
			return err
		}
		logEntry := trx.Logger // initialize a new log entry
		var id string          // unique message identifier
		if msgid := pdu.GetField(smpp.MESSAGE_ID); msgid != nil {
			id = msgid.String()
			logEntry = logEntry.WithField("id", id)
		}
		if status := pdu.GetHeader().Status; status != smpp.ESME_ROK {
			// receive <- status
			logEntry.WithError(status).Error("SMS status with error")
		}
		switch pdu.GetHeader().Id { // look at the message type
		case smpp.SUBMIT_SM_RESP: // message sent by us
			seq := pdu.GetHeader().Sequence // internal number of the sent message
			logEntry.WithField("seq", seq).Info("SMS send response")
			receive <- SendResponse{
				Addr: trx.addr, // server address
				ID:   id,       // external unique message identifier
				Seq:  seq,      // internal message number
			}
		case smpp.DELIVER_SM: // incoming message
			var msg Received    // parsed message
			msg.Addr = trx.addr // server address
			msg.From = pdu.GetField(smpp.SOURCE_ADDR).String()
			msg.To = pdu.GetField(smpp.DESTINATION_ADDR).String()
			txt := pdu.GetField(smpp.SHORT_MESSAGE).ByteArray() // get the raw message text
			datacode := pdu.GetField(smpp.DATA_CODING).Value().(uint8)
			logEntry = logEntry.WithFields(logrus.Fields{
				"from":   msg.From,
				"to":     msg.To,
				"length": len(txt),
				"code":   datacode,
			})
			if classField := pdu.GetField(smpp.ESM_CLASS); classField != nil {
				class := classField.Value().(uint8) // get the message class
				logEntry = logEntry.WithField("class", class)
				if class&0x4 > 0 { // delivery confirmation
					parts := reStatus.FindStringSubmatch(string(txt))
					status := Status{
						Addr:   trx.addr,
						ID:     parts[1],
						Sub:    0,
						Dlvrd:  0,
						Submit: time.Now(),
						Done:   time.Now(),
						Stat:   parts[6],
						Err:    0,
						Text:   parts[8],
					}
					status.Sub, _ = strconv.Atoi(parts[2])
					status.Dlvrd, _ = strconv.Atoi(parts[3])
					status.Submit, _ = time.Parse(statusTimeFormat, parts[4])
					status.Done, _ = time.Parse(statusTimeFormat, parts[5])
					status.Err, _ = strconv.Atoi(parts[7])
					logEntry.WithField("id", status.ID).Infof("SMS status: %q", status.Stat)
					receive <- status
					goto sendResponse
				} else if class&0x40 > 0 { // this is part of a "long" message
					msgs, ok := incomming[txt[3]] // get a reference to the cache
					if !ok {
						msgs = make([][]byte, txt[4])
						incomming[txt[3]] = msgs // save in cache
					}
					msgs[txt[5]-1] = txt[6:]
					logEntry = logEntry.WithFields(logrus.Fields{
						"group": txt[3],
						"total": txt[4],
						"count": txt[5],
					})
					if txt[5] != txt[4] { // received an incomplete message so far
						logEntry.Info("SMS received (part)")
						goto sendResponse
					}
					delete(incomming, txt[3])        // remove from cache
					txt = bytes.Join(msgs, []byte{}) // combine all into a single text
				}
			}
			msg.Text = Decode(datacode, txt)
			logEntry.Info("SMS received (full)")
			logEntry.Debugf("SMS received text: %q", msg.Text)
			receive <- msg
		sendResponse:
			// confirm receipt of the message
			err := trx.DeliverSmResp(pdu.GetHeader().Sequence, smpp.ESME_ROK)
			if err != nil && !trx.isClosed {
				trx.Logger.WithError(err).Error("SMS DeliverSM Response Error")
				// receive <- err
			}
		case smpp.ENQUIRE_LINK_RESP, smpp.ENQUIRE_LINK: // connection confirmation
			continue // ignore
		default: // unhandled message type
			logEntry.WithField("type", pdu.GetHeader().Id).Warning("SMS unsupported command type")
		}
	}
}
