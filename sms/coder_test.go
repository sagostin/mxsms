package sms

import (
	"fmt"
	"testing"
)

func TestGSMCode2(t *testing.T) {
	s := "@[]^^"
	gsm := Encode(3, s)
	utf8 := Decode(3, gsm)
	fmt.Printf("word before: %x\nword after gsm: %x\nword after utf8: %x\n", s, gsm, utf8)
	fmt.Printf("word before: %s\nword after gsm: %s\nword after utf8: %s\n", s, gsm, utf8)
}
