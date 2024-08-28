package sms

import (
	"github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestSmppServer(t *testing.T) {
	logger := logrus.New()
	server := NewSMPPServer("0.0.0.0:2775", "HelloWorld", "HelloWorld", logger)

	err := server.Start()
	if err != nil {
		logger.Fatalf("Failed to start SMPP server: %v", err)
	}

	// The server is now running and handling connections
	time.Sleep(10 * time.Second)

	for s := range server.clients {
		// To send an SMS to a specific client
		err = server.SendSMS(s, "+12508591501", "+17786538344", "Hello, SMPP!")
		if err != nil {
			logger.Errorf("Failed to send SMS: %v", err)
		}
	}

	/*// To broadcast an SMS to all clients
	server.BroadcastSMS("1234567890", "9876543210", "Broadcast message")
	*/
	// Run your main application logic here

	// When you're done:
	server.Stop()
}
