package sms

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"mxsms/smpp"
	"testing"
	"time"
)

func TestSmppServer(t *testing.T) {
	// Define a simple authentication handler
	authHandler := func(systemID, password string) bool {
		// Hard-coded authentication check
		return (systemID == "client1" && password == "pass1") || (systemID == "client2" && password == "pass2")
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
	go func(serv *smpp.Server) {
		for {
			time.Sleep(2 * time.Second) // Simulate delay before sending SMS
			for s, _ := range serv.Clients {
				err := sendMessage(serv, smpp.SMS{
					From:    "+12508591501",
					To:      "+17786538344",
					Message: "Hello from the server!",
					Client:  s,
				})
				logrus.Infof("Sending to %s", s)
				if err != nil {
					logrus.Error(err)
				}
			}
		}
	}(server)

	// Run the server indefinitely (or add your own logic to stop it)
	select {}
}
