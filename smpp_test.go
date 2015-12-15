package main

import (
	"fmt"
	"testing"
	"time"
)

func TestSMPP(t *testing.T) {
	sms := NewSMS("127.0.0.1:2775", "Zultys", "unmQF932")
	defer sms.Close()
	err := sms.Bind()
	if err != nil {
		t.Fatal(err)
	}
	msgID, err := sms.Send("4086751475", "4086751455", fmt.Sprintf("Time message: %q",
		time.Now().Format(time.RFC822)))
	if err != nil {
		t.Error("Send error:", err)
	}
	fmt.Println("MsgID:", msgID)
	time.Sleep(time.Second * 5)
}
