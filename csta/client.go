package csta

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// Client describes the connection to the server and work with it.
type Client struct {
	conn      net.Conn             // connection to the server
	counter   uint32               // counter of sent commands
	keepAlive *time.Timer          // timer for sending keep-alive messages
	mu        sync.Mutex           // lock for shared access
	Logger    *logrus.Entry        // log output
	isClosed  bool                 // flag for closed connection
	handlers  map[Handler]EventMap // additional event handlers
}

// NewClient returns a new initialized client for working with the MX server, which
// operates over an established connection.
func NewClient(conn net.Conn) *Client {
	return &Client{conn: conn, Logger: logrus.NewEntry(logrus.StandardLogger())}
}

// AddHandler adds event handlers that will be used when parsing events
// received from the server.
func (c *Client) AddHandler(handlers ...Handler) {
	c.mu.Lock()
	if c.handlers == nil {
		c.handlers = make(map[Handler]EventMap, len(handlers))
	}
	for _, handler := range handlers {
		c.handlers[handler] = handler.Register(c)
	}
	c.mu.Unlock()
}

// Close closes the connection to the server if it was open.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.isClosed {
		return nil // connection is already closed or hasn't been opened yet
	}
	c.isClosed = true     // set the flag for closed connection
	return c.conn.Close() // close the connection to the server
}

// Send sends a command to the server. As commands, a string or byte array can be passed,
// which already represent a command in XML format. In addition, you can pass any object
// that will be converted to XML format independently. Also, an array of commands in Commands format
// can be passed as a parameter: in this case, all commands from this list will be
// sent in sequence.
func (c *Client) Send(cmd interface{}) (err error) {
	if c.conn == nil || c.isClosed {
		return io.ErrClosedPipe // connection is not established or closed
	}
	// convert the command to XML format
	var dataCmd []byte
	switch data := cmd.(type) {
	case nil: // empty command
		return nil // no command - nothing to send
	case string: // command string
		dataCmd = []byte(data)
	case []byte: // string as a byte array
		dataCmd = data
	case Commands: // list of commands
		for _, cmd := range data { // iterate through all commands in the list
			if err = c.Send(cmd); err != nil { // send the command
				return // break on send error and return it
			}
		}
		return // finished sending all commands
	default:
		if dataCmd, err = xml.Marshal(cmd); err != nil {
			return // return XML serialization error
		}
	}
	// check if there's anything to send
	if dataCmd == nil {
		return nil
	}
	buf := bufPool.Get().(*bytes.Buffer)                 // get buffer from pool
	buf.Reset()                                          // reset buffer
	buf.Write([]byte{0, 0})                              // write message separator
	length := uint16(len(dataCmd) + len(xml.Header) + 8) // calculate message length
	binary.Write(buf, binary.BigEndian, length)          // write length
	counter := atomic.AddUint32(&c.counter, 1)           // increase counter...
	if counter > 9999 {                                  // only 4 digits are allocated for the counter
		atomic.StoreUint32(&c.counter, 1) // reset counter
		counter = 1
	}
	fmt.Fprintf(buf, "%04d", counter) // ...and add it to the message
	buf.WriteString(xml.Header)       // add XML header
	buf.Write(dataCmd)                // add the command text itself
	_, err = buf.WriteTo(c.conn)      // send command to server
	bufPool.Put(buf)                  // return to pool when finished
	if err != nil {                   // in case of error, send it for processing
		return // return error
	}
	if c.keepAlive != nil {
		c.mu.Lock()
		c.keepAlive.Reset(DefaultKeepAliveDuration) // postpone the timer
		c.mu.Unlock()
	}
	c.Logger.WithFields(logrus.Fields{
		"id":       counter,
		"commands": cmd,
	}).Debug("MX command send")
	return nil
}

// Reading starts reading responses from the server and processing them, and also maintains the connection
// using keep-alive messages. In case of correct connection closure, the reading process
// terminates and no error is returned. The process of reading responses from the server is blocking,
// so it is recommended to run it in a separate thread.
func (c *Client) Reading() (err error) {
	// set timer for sending keep-alive messages
	c.mu.Lock()
	c.keepAlive = time.AfterFunc(DefaultKeepAliveDuration, c.sendKeepAlive)
	c.mu.Unlock()
	// at the end, stop the timer and reset the error if the connection was closed through
	// stopping the connection using the Close() method.
	defer func() {
		c.mu.Lock()
		c.keepAlive.Stop() // stop the timer at the end
		c.mu.Unlock()
		if err != nil && c.isClosed {
			err = nil // reset the error description if the connection was correctly closed
		}
	}()
	header := make([]byte, 8) // response header
	for {                     // infinite loop for reading all messages from the stream
		// read response header
		if _, err := io.ReadFull(c.conn, header); err != nil {
			return err // return read error
		}
		// get message length, allocate memory for it and read the message itself
		data := make([]byte, binary.BigEndian.Uint16(header[2:4])-8)
		if _, err := io.ReadFull(c.conn, data); err != nil {
			return err // return read error
		}
		// server command identifier
		id, err := strconv.Atoi(string(header[4:]))
		if err != nil {
			c.Logger.WithError(err).Debug("MX ignore message with bad ID")
			continue // skip command with unclear number
		}
		// initialize XML decoder, get event name and data
		xmlDecoder := xml.NewDecoder(bytes.NewReader(data))
	readingToken:
		offset := xmlDecoder.InputOffset() // save offset from the beginning
		token, err := xmlDecoder.Token()   // read XML element name
		if err != nil {
			if err != io.EOF { // output to log
				c.Logger.WithError(err).Debug("MX ignore error token")
			}
			continue // ignore messages with invalid XML - read next message
		}
		// find the starting XML element, and skip everything else
		startToken, ok := token.(xml.StartElement)
		if !ok { // if it's not the root XML element, move to the next one
			goto readingToken
		}
		eventName := startToken.Name.Local // get event name
		c.Logger.WithFields(logrus.Fields{
			"id":   id,
			"data": string(data[offset:]),
		}).Debug("MX event received")
		// processing internal events
		if eventData := defaultClientEvents.GetDataPointer(eventName); eventData != nil {
			// parse the data returned in the event description
			if err := xmlDecoder.DecodeElement(eventData, &startToken); err != nil {
				return err
			}
			// process parsed data
			switch data := eventData.(type) {
			case *ErrorCode: // CSTA Error
				return data // return as error
			case *LoginResponse: // login information
				if data.Code != 0 {
					return data // return as error
				}
			}
		}
		// iterate through all message handlers
		for handler, eventMap := range c.handlers {
			// get pointer to the data structure for parsing the event
			eventData := eventMap.GetDataPointer(eventName)
			if eventData == nil {
				continue // skip unsupported events
			}
			// parse the data returned in the event description
			if err := xmlDecoder.DecodeElement(eventData, &startToken); err != nil {
				c.Logger.WithError(err).WithField("event", eventName).
					Debug("MX ignore decode XML error")
				continue // ignore elements that couldn't be parsed
			}
			// pass the parsed event for processing
			if err := handler.Handle(eventData); err != nil {
				return fmt.Errorf("%s: %v", eventName, err)
			}
		}
	}
}

// Login sends an authorization command to the server.
func (c *Client) Login(login Login) error {
	return c.Send(login.loginRequest())
}

// sendKeepAlive sends a keep-alive message to the server as soon as the timer triggers
func (c *Client) sendKeepAlive() {
	if c.conn == nil || c.isClosed {
		return
	}
	// send a pre-prepared keep-alive command to the server.
	// I just didn't want to encode it into binary form every time, so it was done
	// only once, and now it's sent in an already prepared form.
	c.conn.Write([]byte{0x00, 0x00, 0x00, 0x15, 0x30, 0x30, 0x30, 0x30, 0x3c,
		0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x20, 0x2f, 0x3e})
	if c.keepAlive != nil {
		c.mu.Lock()
		c.keepAlive.Reset(DefaultKeepAliveDuration) // postpone the timer to a later time
		c.mu.Unlock()
	}
}

// pool of buffers used when forming commands for the server.
var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer) // initialize and return a new buffer
	},
}
