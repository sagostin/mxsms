package csta

import (
	"bytes"
	"fmt"

	"github.com/mdigger/log"
)

// LogINOUT задает символы, используемые для вывода направления
// (true - входящие, false - исходящие)
var LogINOUT = map[bool]string{true: "→", false: "←"}

// LogLevel задает уровень для вывода в лог.
var LogLevel = log.TRACE + 16

// log форматирует вывод лога с командами CSTA.
func (c *Conn) log(inFlag bool, id uint16, data []byte) {
	c.mul.RLock()
	if c.logger == nil {
		c.mul.RUnlock()
		return
	}
	var name = data
	if indx := bytes.IndexAny(data, " />"); indx > 1 {
		name = data[1:indx]
		if bytes.Equal(name, []byte("close")) {
			data = nil
		}
	}
	// вырезаем содержимое файлов голосовой почты
	if start := bytes.Index(data, []byte("<mediaContent>")); start > 0 {
		if end := bytes.LastIndex(data, []byte("</mediaContent>")); end > 0 {
			b := make([]byte, start+14+len(data)-end+15)
			bp := copy(b, data[:start+14])
			bp += copy(b[bp:], []byte("[base64 encoded]"))
			copy(b[bp:], data[end:])
			data = b
		}
	}
	var msg = fmt.Sprintf("%s %s", LogINOUT[inFlag], name)
	if id > 0 && id < 9999 {
		c.logger.Log(LogLevel, msg, "id", fmt.Sprintf("%04d", id), "xml", string(data))
	} else if data != nil {
		c.logger.Log(LogLevel, msg, "xml", string(data))
	} else {
		c.logger.Log(LogLevel, msg)
	}
	c.mul.RUnlock()
}

// SetLogger устанавливает лог.
func (c *Conn) SetLogger(l *log.Logger) {
	c.mul.Lock()
	c.logger = l
	c.mul.Unlock()
}
