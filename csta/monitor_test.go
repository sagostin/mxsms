package csta

import (
	"fmt"
	"testing"
)

func TestMonitor(t *testing.T) {
	conn, err := Connect("89.185.246.134:7778")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.Login(Login{
		UserName: "peterh",
		Password: "981211",
		Type:     "User",
		Platform: "iPhone",
		Version:  "1.0",
	}); err != nil {
		t.Fatal(err)
	}

	ab, err := conn.Addressbook()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		for _, contact := range ab {
			if id, err := conn.MonitorStart(contact.Ext); err != nil {
				t.Error(err)
			} else {
				fmt.Println("Monitor started:", contact.Ext, id)
			}
		}
	}
	// pretty.Println(conn.monitors)

	for _, contact := range ab {
		fmt.Println("Monitors:", contact.Ext, conn.Monitor(contact.Ext))
	}

	for _, contact := range ab {
		if err = conn.MonitorStop(contact.Ext); err != nil {
			t.Error(err)
		}
	}

	// pretty.Println(conn.monitors)

	if err = conn.MonitorStop("444"); err != nil {
		t.Error(err)
	}
	if err = conn.MonitorStopID(999); err != nil {
		t.Error(err)
	}

}
