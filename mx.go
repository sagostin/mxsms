package main

import (
	"github.com/sirupsen/logrus"
)

// PhoneInfo describes rules for parsing phone numbers
type PhoneInfo struct {
	Short  int               `yaml:",omitempty"`              // length of short phone number
	Prefix string            `yaml:"defaultPrefix,omitempty"` // prefix for incomplete phone number
	From   map[string]string // list of outgoing phone numbers
}

// MX describes the service configuration, including necessary data for connecting to the server
// and user authorization.
type MX struct {
	name           string                        // service name
	csta_old.Addr  `yaml:"server" json:"server"` // server address
	csta_old.Login `json:"login"`                // authorization information
	PhoneInfo      `yaml:"phones" json:"phones"` // information for parsing phone numbers
	DefaultJID     string                        `yaml:"defaultJID,omitempty" json:"defaultJID,omitempty"` // where to deliver unknown messages
	Disabled       bool                          `yaml:",omitempty" json:"disabled,omitempty"`             // flag for ignored service
	Logger         *logrus.Entry                 `yaml:"-"`                                                // log for outputting service information
	handler        *MessageHandle                // chat message handler
	client         *csta_old.Client              // client for connection to MX-server
}

// Connect establishes a connection and starts the service.
func (mx *MX) Connect() error {
	if mx.Logger == nil { // initialize log support
		mx.Logger = logrus.NewEntry(logrus.StandardLogger())
	}
	if mx.name != "" { // add server name to the log if it's defined
		mx.Logger = mx.Logger.WithField("mx", mx.name)
	}
	if mx.Disabled {
		mx.Logger.Warning("Ignore disabled")
		return nil
	}
	conn, err := mx.Addr.Dial()
	if err != nil {
		mx.Logger.WithError(err).Error("MX Connecting error")
		return err // return error of establishing connection with the server
	}
	mx.Logger.WithField("host", mx.Addr.FullAddr()).Info("MX Connected")
	// initialize the client
	client := csta_old.NewClient(conn)
	defer client.Close()
	client.Logger = mx.Logger
	// initialize message handler
	mx.handler = NewMessageHandler(config.SMSGate, mx)
	client.AddHandler(mx.handler)
	if err := client.Login(mx.Login); err != nil {
		mx.Logger.WithError(err).Error("Authorizing error")
		return err // error sending authorization command to the server
	}
	mx.Logger.WithField("login", mx.Login.User).Info("MX Authorized")
	mx.client = client
	// start the process of reading responses from the server
	err = client.Reading()
	if err != nil {
		mx.Logger.WithError(err).Error("MX error")
	}
	return err
}

// Close stops the running service.
func (mx *MX) Close() error {
	if mx.client == nil {
		return nil // client connection to the server is not initialized
	}
	mx.Logger.Info("MX Close")
	return mx.client.Close() // stop connection to the server
}
