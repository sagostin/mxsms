package sms

import (
	"bytes"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	utf8GsmChars = map[rune]string{
		// ... (map content remains unchanged)
	}

	gsmUtf8Chars = map[rune]string{
		// ... (map content remains unchanged)
	}
)

func Decode(code uint8, text []byte) string {
	switch code {
	case 8: // UCS2
		es, _, _ := transform.Bytes(
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), text)
		return string(es)
	case 3: // latin1 (windows1252)
		es, _, _ := transform.Bytes(charmap.Windows1252.NewDecoder(), text)
		return string(es)
	case 0: // decode from GSM 03.38 format
		var result bytes.Buffer
		for _, r := range text {
			if nr, ok := gsmUtf8Chars[rune(r)]; ok { // make replacements for known symbols
				result.WriteString(nr)
				continue
			}
			result.WriteByte(r) // add as is
		}
		return result.String()
	default:
		return string(text)
	}
}

func Encode(code uint8, text string) []byte {
	switch code { // depending on the suitable encoding, choose the corresponding encoding method
	case 8: // ucs8
		enc := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
		es, _, _ := transform.Bytes(enc, []byte(text))
		return es
	case 3: // latin1
		es, _, _ := transform.Bytes(charmap.Windows1252.NewEncoder(), []byte(text))
		return es
	case 0: // encode to GSM 03.38
		var result bytes.Buffer
		for _, r := range text {
			if nr, ok := utf8GsmChars[r]; ok { // make replacements for known symbols
				result.WriteString(nr)
				continue
			}
			if r > '\u007F' { // remove everything that doesn't fit the format
				result.WriteRune('?')
				continue
			}
			result.WriteRune(r) // add as is
		}
		return result.Bytes()
	default:
		return []byte(text)
	}
}
