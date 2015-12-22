package main

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/mdigger/mxsms2/csta"
	"github.com/mdigger/mxsms2/sms"
	"gopkg.in/yaml.v2"
)

func TestConfigGenerate(t *testing.T) {
	var config = &Config{
		MX: map[string]*MX{
			"xyzrd-test": {
				Addr: csta.Addr{
					Host:           "10.30.2.221",
					Port:           7778,
					Secure:         true,
					SkipVerify:     true,
					Timeout:        time.Second * 20,
					ReconnectDelay: time.Second * 30,
					MaxError:       5,
				},
				Login: csta.Login{
					User:     "smsgate",
					Password: "9185",
				},
				PhoneInfo: PhoneInfo{
					Short:  0,
					Prefix: "1",
					From:   []string{"14086751455", "14086751475"},
				},
				DefaultJID: "44086340573989457",
			},
		},
		SMSGate: &SMSGate{
			SMPP: &sms.SMPP{
				Address:         []string{"67.231.1.30:2775", "67.231.4.201:2775"},
				SystemID:        "Zultys",
				Password:        "unmQF932",
				MaxParts:        8,
				EnquireDuration: time.Second * 30,
				ReconnectDelay:  time.Second * 30,
				MaxError:        5,
			},
			Responses: SMSTemplates{
				NoPhone:   "No phone in the beginning of the message",
				Incorrect: "Invalid phone number: %q",
				Accepted:  "SMS sended to %q",
				Delivered: "SMS delivered to %q",
				Error:     "SMS send error: %s",
				Incoming:  "SMS from %q\n%s",
			},
		},
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(data))
	err = ioutil.WriteFile("config.yaml", data, 0666)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigFile(t *testing.T) {
	config, err := LoadConfig("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pretty.Println(config)
}
