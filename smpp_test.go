package main

import (
	"fmt"
	"testing"
	"time"
)

func TestSMPP(t *testing.T) {
	sms := NewSMS("67.231.4.201:2775", "Zultys", "unmQF932")
	defer sms.Close()
	err := sms.Bind()
	if err != nil {
		t.Fatal(err)
	}
	msgID, err := sms.Send("4086751475", "4086751455", fmt.Sprintf("Time message: %q",
		time.Now().Format(time.RFC822)))
	if err != nil {
		fmt.Println("Send error:", err)
		// t.Error("Send error:", err)
	} else {
		fmt.Println("MsgID:", msgID)
	}
	time.Sleep(time.Second * 5)
}
