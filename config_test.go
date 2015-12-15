package main

import (
	"testing"

	"github.com/kr/pretty"
)

func TestConfigFile(t *testing.T) {
	config, err := LoadConfig("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pretty.Println(config)
}
