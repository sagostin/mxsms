package main

import (
	"fmt"
	"testing"
	"time"
)

func TestSMPP(t *testing.T) {
	sms := NewSMS("67.231.4.201:2775", "Zultys", "unmQF932")
	defer sms.Close()
	msgID, err := sms.Send("4086751455", "4086751475", "Test message")
	if err != nil {
		t.Error("Send error:", err)
	}
	fmt.Println("MsgID:", msgID)
	time.Sleep(time.Second * 30)
}
