package sms

import (
	"bytes"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	utf8GsmChars = map[rune]string{
		'@': "\x00", '£': "\x01", '$': "\x02",
		'¥': "\x03", 'è': "\x04", 'é': "\x05",
		'ù': "\x06", 'ì': "\x07", 'ò': "\x08",
		'Ç': "\x09", 'Ø': "\x0B", 'ø': "\x0C",
		'Å': "\x0E", 'Δ': "\x10", '_': "\x11",
		'Φ': "\x12", 'Γ': "\x13", 'Λ': "\x14",
		'Ω': "\x15", 'Π': "\x16", 'Ψ': "\x17",
		'Σ': "\x18", 'Θ': "\x19", 'Ξ': "\x1A",
		'Æ': "\x1C", 'æ': "\x1D", 'ß': "\x1E",
		'É': "\x1F", 'Ä': "\x5B", 'Ö': "\x5C",
		'Ñ': "\x5D", 'Ü': "\x5E", '§': "\x5F",
		'¿': "\x60", 'ä': "\x7B", 'ö': "\x7C",
		'ñ': "\x7D", 'ü': "\x7E", 'à': "\x7F",

		'^': "\x1B\x14", '{': "\x1B\x28",
		'}': "\x1B\x29", '\\': "\x1B\x2F",
		'[': "\x1B\x3C", '~': "\x1B\x3D",
		']': "\x1B\x3E", '|': "\x1B\x40",
		'€': "\x1B\x65",
	}

	gsmUtf8Chars = map[rune]string{
		'\x00': "\x40",
		'\x01': "\xC2\xA3",
		'\x02': "\x24",
		'\x03': "\xC2\xA5",
		'\x04': "\xC3\xA8",
		'\x05': "\xC3\xA9",
		'\x06': "\xC3\xB9",
		'\x07': "\xC3\xAC",
		'\x08': "\xC3\xB2",
		'\x09': "\xC3\x87",
		'\x0B': "\xC3\x98",
		'\x0C': "\xC3\xB8",
		'\x0E': "\xC3\xB8",
		'\x0F': "\xC3\xA5",
		'\x10': "\xCE\x94",
		'\x11': "\x5F",
		'\x12': "\xCE\xA6",
		'\x13': "\xCE\x93",
		'\x14': "\xCE\xA0",
		'\x15': "\xCE\xA9",
		'\x16': "\xCE\xA0",
		'\x17': "\xCE\xA8",
		'\x18': "\xCE\xA3",
		'\x19': "\xCE\x98",
		'\x1A': "\xCE\x9E",
		'\x1C': "\xC3\x86",
		'\x1D': "\xC3\xA6",
		'\x1E': "\xC3\x9F",
		'\x1F': "\xC3\x89",
		'\x20': "\x20",
		'\x24': "\xC2\xA4",
		'\x40': "\xC2\xA1",
		'\x5B': "\xC3\x84",
		'\x5C': "\xC3\x96",
		'\x5D': "\xC3\x91",
		'\x5E': "\xC3\x9C",
		'\x5F': "\xC2\xA7",
		'\x60': "\xC2\xBF",
		'\x7B': "\xC3\xA8",
		'\x7C': "\xC3\xB6",
		'\x7D': "\xC3\xB1",
		'\x7E': "\xC3\xBC",
		'\x7F': "\xC3\xA0",
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
