package csta_old

import (
	"fmt"
	"reflect"
)

// Handler describes the interface for event handlers
type Handler interface {
	Register(*Client) EventMap
	Handle(interface{}) error
}

// EventMap describes a list of supported event names and associated data structures
// for parsing them. It is used when parsing events received from the server.
type EventMap map[string]reflect.Type

// GetDataPointer returns a pointer to an empty structure that supports parsing event data
// with the name specified in the parameter. Returns nil if the event is not supported.
func (em EventMap) GetDataPointer(eventName string) interface{} {
	dataType, ok := em[eventName] // get the structure for parsing the event
	if !ok || dataType == nil {
		return nil // event with this name is not supported
	}
	// get a pointer to the data structure for parsing the event
	return reflect.New(dataType).Interface()
}

// defaultClientEvents describes the list of default supported Client events.
var defaultClientEvents = EventMap{
	"CSTAErrorCode": reflect.TypeOf(ErrorCode{}),
	"loginResponce": reflect.TypeOf(LoginResponse{}),
	"loginFailed":   reflect.TypeOf(LoginResponse{}),
}

// Commands describes a list of returned commands.
type Commands []interface{}

// Add adds a new command to the list.
func (c *Commands) Add(cmd interface{}) {
	if str, ok := cmd.(string); (ok && str == "") || (cmd == nil) {
		return // ignore empty commands
	}
	if c == nil { // initialize the list if it was not initialized
		*c = make([]interface{}, 0)
	}
	*c = append(*c, cmd) // add the command to the list
}

// ErrorCode (CSTAErrorCode) describes information about a CSTA error.
type ErrorCode struct {
	Message string `xml:",any"`
}

// Error returns a string with the error description.
func (e *ErrorCode) Error() string {
	return fmt.Sprintf("CSTA error: %s", e.Message)
}
