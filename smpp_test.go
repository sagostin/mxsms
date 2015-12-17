package main

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestSMPP(t *testing.T) {
	smpp := &SMPP{Address: []string{"67.231.4.201:2775"}, User: "Zultys", Password: "unmQF932"}
	if err := smpp.Connect(); err != nil {
		t.Fatal(err)
	}
	defer smpp.Close()
	msgID, err := smpp.Send("+14086751475", "+14154292837", fmt.Sprintf("Time message: %q",
		time.Now().Format(time.RFC822)))
	if err != nil {
		log.Println("Send error:", err)
	} else {
		log.Println("MsgID:", msgID)
	}
	time.Sleep(time.Second * 5)
	log.Println("THE END")
}
