package sms

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"math/rand"
	"mxsms/smpp" // replace this with the actual import path to your smpp package
	"time"
)

func main() {
	// Define a simple authentication handler
	authHandler := func(systemID, password string) bool {
		// Hard-coded authentication check
		return (systemID == "client1" && password == "pass1") || (systemID == "client2" && password == "password2")
	}

	// Create the SMPP server
	server := smpp.NewServer("0.0.0.0:2776", authHandler)

	// Start the server
	server.Start()
	/*if err != nil {
		log.Fatalf("Failed to start SMPP server: %v", err)
	}*/
	fmt.Println("SMPP server started on :2776")

	// Start the inbound processing queue
	go processIncomingMessages(server.IncomingChannel)

	// Start the outbound processing queue
	/*go processOutgoingMessages(server.OutgoingChannel, server)*/

	// Example of sending a message to a specific client after 10 seconds
	go func() {
		time.Sleep(10 * time.Second) // Simulate delay before sending SMS
		err := sendMessage(server, smpp.SMS{
			From:    "Server",
			To:      "client1",
			Message: "Hello from the server!",
		})
		if err != nil {
			logrus.Error(err)
		}
		fmt.Println("Queued SMS to client1 for sending")
	}()

	// Run the server indefinitely (or add your own logic to stop it)
	select {}
}

// Process incoming messages (Inbound Queue)
func processIncomingMessages(incomingQueue <-chan smpp.SMS) {
	for sms := range incomingQueue {
		// Process the received SMS message
		fmt.Printf("Received SMS from %s to %s: %s\n", sms.From, sms.To, sms.Message)
		// Add custom processing logic here (e.g., saving to a database, triggering an action)
	}
}

/*// Process outgoing messages (Outbound Queue)
func processOutgoingMessages(outgoingQueue <-chan smpp.SMS, server *smpp.Server) {
	for sms := range outgoingQueue {
		// Process and send the SMS message
		server.SendSMS(sms) // Utilize the sendSMS method in the Server struct to send the message
		fmt.Printf("Sent SMS from %s to %s: %s\n", sms.From, sms.To, sms.Message)
		// Add custom logic here (e.g., logging, notification of successful sending)
	}
}*/

func sendMessage(s *smpp.Server, sms smpp.SMS) error {
	session := s.Clients[sms.Client]

	text := sms.Message
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
	// check if the message fits into one
	if len(text) <= maxOneMessageLength {
		p, err := session.SubmitSm(sms.From, sms.To, text, params) // send as is
		if err == nil {
			//sms.Seq = []uint32{seq}
		}
		if err := session.Write(p); err != nil {
			return err
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
	//sms.Seq = make([]uint32, 0, count) // initialize the list of identifiers
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
		/*logEntry.WithFields(logrus.Fields{
			"count":  i + 1,
			"total":  count,
			"length": len(msg),
		}).Info("SMS send")*/

		p, err := session.SubmitSm(sms.From, sms.To, msg, params) // send as is
		if err == nil {
			//sms.Seq = []uint32{seq}
		}
		if err := session.Write(p); err != nil {
			return err
		}
		//sms.Seq = append(sms.Seq, seq)
	}
	return nil
}
