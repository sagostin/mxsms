package sms

import "time"

// Received describes a delivered and parsed SMS message.
// It includes only those fields that were of interest to me.
type Received struct {
	From string // from which number
	To   string // to which number
	Text string // message text (already decoded)
	Addr string // SMPP server identifier
}

type SendMessage struct {
	MXName string   // name of the MX server from the configuration
	JID    string   // unique user identifier in MX
	From   string   // from which number
	To     string   // to which number
	Text   string   // message text (already decoded)
	Seq    []uint32 // internal numbers of sent messages
}

type SendResponse struct {
	ID   string // message identifier
	Seq  uint32 // internal message number
	Addr string // SMPP server identifier
}

type Status struct {
	ID     string    // message identifier
	Sub    int       // number of SMS parts
	Dlvrd  int       // number of delivered parts
	Submit time.Time // message send date
	Done   time.Time // date when the message reached its final state
	Stat   string    // Delivery status message_state in string form
	Err    int       // Extended delivery status network_error_code
	Text   string    // Text representation
	Addr   string    // SMPP server identifier
}
