package main

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/mdigger/mxsms3/csta"

	"gopkg.in/yaml.v2"
)

func TestConfigGenerate(t *testing.T) {
	var config = &Config{
		MX: map[string]*MX{
			"xyzrd": {
				Addr: csta.Addr{
					Host:           "voip.xyzrd.com",
					Port:           7778,
					Secure:         true,
					SkipVerify:     true,
					Timeout:        time.Second * 10,
					ReconnectDelay: time.Second * 30,
				},
				Login: csta.Login{
					User:     "d3",
					Password: "9185",
				},
				PhoneInfo: PhoneInfo{
					Short:  0,
					Prefix: "7",
					From:   []string{"14086751455", "14086751475"},
				},
			},
		},
		SMPP: &SMPP{
			Address:         []string{"67.231.1.30:2775", "67.231.4.201:2775"},
			SystemID:        "Zultys",
			Password:        "unmQF932",
			EnquireDuration: time.Minute,
			MaxParts:        3,
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
