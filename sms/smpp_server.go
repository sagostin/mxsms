package sms

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mdigger/smpp"
	"github.com/sirupsen/logrus"
)

// SMPPServer represents the SMPP server
type SMPPServer struct {
	Address   string
	SystemID  string
	Password  string
	Logger    *logrus.Logger
	clients   map[string]*Transceiver
	mu        sync.RWMutex
	listener  net.Listener
	receiveCh chan interface{}
}

// NewSMPPServer creates a new SMPP server instance
func NewSMPPServer(address, systemID, password string, logger *logrus.Logger) *SMPPServer {
	return &SMPPServer{
		Address:   address,
		SystemID:  systemID,
		Password:  password,
		Logger:    logger,
		clients:   make(map[string]*Transceiver),
		receiveCh: make(chan interface{}, 1000),
	}
}

// Start starts the SMPP server
func (s *SMPPServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.Address)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	s.Logger.Infof("SMPP Server started on %s", s.Address)

	go s.acceptConnections()
	go s.handleIncomingMessages()

	return nil
}

func (s *SMPPServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.Logger.Errorf("Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *SMPPServer) handleConnection(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	logEntry := s.Logger.WithField("remote", addr)

	bindParams := smpp.Params{
		smpp.SYSTEM_ID: s.SystemID,
		smpp.PASSWORD:  s.Password,
	}

	trx, err := NewTransceiver(addr, 30*time.Second, bindParams, logEntry.WithFields(nil))
	if err != nil {
		logEntry.Errorf("Failed to create transceiver: %v", err)
		conn.Close()
		return
	}

	s.mu.Lock()
	s.clients[addr] = trx
	s.mu.Unlock()

	// Start reading messages from this client
	go trx.reading(s.receiveCh)

	logEntry.Info("New client connected")
}

func (s *SMPPServer) handleIncomingMessages() {
	for msg := range s.receiveCh {
		switch m := msg.(type) {
		case Received:
			s.Logger.WithFields(logrus.Fields{
				"from": m.From,
				"to":   m.To,
				"text": m.Text,
			}).Info("Received SMS")
			// Process the received message
		case SendResponse:
			s.Logger.WithFields(logrus.Fields{
				"id":  m.ID,
				"seq": m.Seq,
			}).Info("SMS sent")
			// Process the send response
		case Status:
			s.Logger.WithFields(logrus.Fields{
				"id":   m.ID,
				"stat": m.Stat,
			}).Info("SMS status update")
			// Process the status update
		case error:
			s.Logger.Errorf("Error received: %v", m)
			// Handle the error, possibly closing the connection if necessary
		default:
			s.Logger.Warnf("Received unknown message type: %T", m)
		}
	}
}

// SendSMS sends an SMS through the appropriate client transceiver
func (s *SMPPServer) SendSMS(clientAddr, from, to, text string) error {
	s.mu.RLock()
	trx, exists := s.clients[clientAddr]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no client found for address: %s", clientAddr)
	}

	sms := &SendMessage{
		From: from,
		To:   to,
		Text: text,
	}

	return trx.Send(sms)
}

// BroadcastSMS sends an SMS to all connected clients
func (s *SMPPServer) BroadcastSMS(from, to, text string) {
	sms := &SendMessage{
		From: from,
		To:   to,
		Text: text,
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, trx := range s.clients {
		go func(t *Transceiver) {
			if err := t.Send(sms); err != nil {
				s.Logger.WithError(err).Errorf("Failed to send SMS to client %s", t.addr)
			}
		}(trx)
	}
}

// Stop stops the SMPP server and closes all client connections
func (s *SMPPServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	for _, trx := range s.clients {
		trx.Close()
	}
	s.clients = make(map[string]*Transceiver)
	s.mu.Unlock()

	close(s.receiveCh)

	s.Logger.Info("SMPP Server stopped")
}
