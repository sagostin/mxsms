package sms

import (
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func Decode(code uint8, text []byte) string {
	switch code {
	case 8: // UCS2
		es, _, err := transform.Bytes(
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), text)
		if err == nil {
			return string(es)
		}
	case 3: // latin1 (windows1252)
		es, _, err := transform.Bytes(charmap.Windows1252.NewDecoder(), text)
		if err == nil {
			return string(es)
		}
	}
	return string(text)
}

func Encode(code uint8, text []byte) string {
	switch code { // в зависимости от подходящей кодировки выбираем соответствующий метод кодирования
	case 8: // ucs8
		enc := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
		if es, _, err := transform.Bytes(enc, text); err == nil {
			return string(es)
		}
	case 3: // latin1
		es, _, err := transform.Bytes(charmap.Windows1252.NewEncoder(), text)
		if err == nil {
			return string(es)
		}
	}
	return string(text)
}
