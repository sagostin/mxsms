package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"time"
)

type Config struct {
	MX      map[string]*MX `json:"mx,omitempty"`      // MX servers
	SMSGate *SMSGate       `json:"smsgate,omitempty"` // SMS handling settings
}

// ParseConfig parses the configuration and initializes initial values.
func ParseConfig(data []byte) (*Config, error) {
	config := new(Config)
	err := json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	for name, mx := range config.MX {
		if mx.Disabled {
			delete(config.MX, name) // immediately remove blocked MX servers
			continue
		}
		mx.name = name                                            // save the server configuration name
		mx.Logger = logrus.StandardLogger().WithField("mx", name) // assign log handler
	}
	return config, nil
}

// LoadConfig loads and parses the configuration from a file.
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseConfig(data)
}

func (c *Config) MXConnect() {
	for _, mx := range c.MX {
		go func(mx *MX) {
			maxErrors := MaxErrors // set the maximum number of allowable errors
			if mx.Addr.MaxError > 0 {
				maxErrors = mx.Addr.MaxError
			}
			var lastErrorTime time.Time      // time when the last temporary error occurred
			for i := 0; i < maxErrors; i++ { // restart the service automatically in case of connection errors
				err := mx.Connect() // establish connection with the server
				//zabbixLog.Send("mx.sms.link.lost", mx.name)
				switch err := err.(type) { // check the error type
				// case *csta.ErrorCode, *csta.LoginResponse, syscall.Errno:
				case *net.OpError:
					if !(err.Temporary() || err.Timeout()) {
						break // this is not a temporary error
					}
					if time.Since(lastErrorTime) > time.Minute*30 {
						i = 0 // reset error counter if they were long ago
					}
					reconnectDelay, _ := time.ParseDuration(mx.Addr.ReconnectDelay)
					time.Sleep(reconnectDelay) // delay before next attempt
					lastErrorTime = time.Now() // remember error time
					continue                   // non-critical errors - re-establish connection
				case nil:
					return // planned service stop
				}
				break
			}
			mx.Logger.Warning("MX connection stopped")
			return // service stop
		}(mx) // isolate the service as a parameter, otherwise only the last one will be launched
	}
}

func (c *Config) MXClose() {
	for _, mx := range c.MX { // stop all running connections to MX
		mx.Close()
	}
}
