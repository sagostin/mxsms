package smpp

import (
	"crypto/tls"
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type SMS struct {
	From    string
	To      string
	Message string
	Client  string
}

type Server struct {
	addr            string
	tlsConfig       *tls.Config
	Clients         map[string]*ClientSession
	clientAddresses map[string]string // address -> clientId/systemId
	clientsMu       sync.RWMutex
	listener        net.Listener
	authHandler     func(systemID, password string) bool
	IncomingChannel chan SMS
	OutgoingChannel chan SMS
	pendingReceipts map[string]string
	receiptsMutex   sync.Mutex
}

type ClientSession struct {
	*Smpp
	systemID string
	bound    bool
}

func NewServer(addr string, authHandler func(systemID, password string) bool) *Server {
	return &Server{
		addr:            addr,
		Clients:         make(map[string]*ClientSession),
		clientAddresses: make(map[string]string),
		authHandler:     authHandler,
		IncomingChannel: make(chan SMS, 100),
		OutgoingChannel: make(chan SMS, 100),
		pendingReceipts: make(map[string]string),
	}
}

func NewServerTLS(addr string, config *tls.Config, authHandler func(systemID, password string) bool) *Server {
	server := NewServer(addr, authHandler)
	server.tlsConfig = config
	return server
}

func (s *Server) Start() {
	var err error

	if s.tlsConfig != nil {
		s.listener, err = tls.Listen("tcp", s.addr, s.tlsConfig)
	} else {
		s.listener, err = net.Listen("tcp", s.addr)
	}
	if err != nil {
		fmt.Printf("failed to start server: %v", err)
	}

	go s.acceptConnections()
}

func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			logrus.Error("unable to accept connection")
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	smpp := &Smpp{conn: conn}
	session := &ClientSession{Smpp: smpp}

	defer func() {
		session.Close()
		s.removeClient(session)
	}()

	if err := s.bindClient(session); err != nil {
		return
	}

	for {
		pdu, err := session.Read()
		if err != nil {
			break
		}
		s.handlePDU(session, pdu)
	}
}

func (s *Server) bindClient(session *ClientSession) error {
	pdu, err := session.Read()
	if err != nil {
		return err
	}

	bindPdu, ok := pdu.(*Bind)
	if !ok {
		return fmt.Errorf("expected Bind PDU, got %T", pdu)
	}

	systemID := bindPdu.GetField(SYSTEM_ID).String()
	password := bindPdu.GetField(PASSWORD).String()

	if !s.authHandler(systemID, password) {
		resp, _ := session.BindResp(BIND_TRANSCEIVER_RESP, bindPdu.GetHeader().Sequence, ESME_RINVPASWD, "")
		session.Write(resp)
		return fmt.Errorf("authentication failed")
	}

	resp, _ := session.BindResp(BIND_TRANSCEIVER_RESP, bindPdu.GetHeader().Sequence, ESME_ROK, systemID)
	if err := session.Write(resp); err != nil {
		return err
	}

	session.systemID = systemID
	session.bound = true
	s.addClient(systemID, session)
	return nil
}

func (s *Server) handlePDU(session *ClientSession, pdu Pdu) {
	switch pdu.GetHeader().Id {
	case SUBMIT_SM:
		s.isAuthenticated(session)
		s.handleSubmitSM(session, pdu)
	case DELIVER_SM_RESP:
		s.isAuthenticated(session)
		s.handleDeliverSmResp(session, pdu)
	case ENQUIRE_LINK:
		s.isAuthenticated(session)
		s.handleEnquireLink(session, pdu)
	case UNBIND:
		s.handleUnbind(session, pdu)
	default:
		s.handleUnknownPDU(session, pdu)
	}
}

func (s *Server) handleSubmitSM(session *ClientSession, pdu Pdu) {
	submitSM, ok := pdu.(*SubmitSm)
	if !ok {
		fmt.Printf("Error: Expected SubmitSm, got %T\n", pdu)
		return
	}

	messageID := generateMessageID()

	registeredDelivery := submitSM.GetField(REGISTERED_DELIVERY).Value().(uint8)
	if registeredDelivery&0x01 != 0 {
		s.receiptsMutex.Lock()
		s.pendingReceipts[messageID] = session.systemID
		s.receiptsMutex.Unlock()
	}

	s.IncomingChannel <- SMS{
		From:    submitSM.GetField(SOURCE_ADDR).String(),
		To:      submitSM.GetField(DESTINATION_ADDR).String(),
		Message: submitSM.GetField(SHORT_MESSAGE).String(),
	}

	resp, _ := session.SubmitSmResp(submitSM.GetHeader().Sequence, ESME_ROK, messageID)
	session.Write(resp)

	if registeredDelivery&0x01 != 0 {
		go s.simulateDeliveryAndSendReceipt(submitSM, messageID)
	}
}

func (s *Server) simulateDeliveryAndSendReceipt(submitSM *SubmitSm, messageID string) {
	time.Sleep(5 * time.Second)

	s.receiptsMutex.Lock()
	systemID, exists := s.pendingReceipts[messageID]
	delete(s.pendingReceipts, messageID)
	s.receiptsMutex.Unlock()

	if !exists {
		return
	}

	s.clientsMu.RLock()
	session, exists := s.Clients[systemID]
	s.clientsMu.RUnlock()

	if !exists {
		return
	}
	/*	receiptMsg := fmt.Sprintf("id:%s submit date:%s done date:%s stat:DELIVRD err:000 text:%s",
		messageID,
		time.Now().Add(-5*time.Second).Format("0601021504"),
		time.Now().Format("0601021504"),
		submitSM.GetField(SHORT_MESSAGE).String()[:20])*/

	// Respond back to Deliver SM with Deliver SM Resp
	resp, err := session.DeliverSmResp(submitSM.Sequence+1, ESME_ROK)
	session.Write(resp)
	if err != nil {
		fmt.Println("DeliverSmResp err:", err)
	}
}

func (s *Server) handleDeliverSmResp(session *ClientSession, pdu Pdu) {
	deliverSmResp, ok := pdu.(*DeliverSmResp)
	if !ok {
		fmt.Printf("Error: Expected DeliverSmResp, got %T\n", pdu)
		return
	}

	if deliverSmResp.Ok() {
		fmt.Printf("Delivery receipt acknowledged by client %s\n", session.systemID)
	} else {
		fmt.Printf("Delivery receipt failed for client %s with status %d\n", session.systemID, deliverSmResp.Header.Status)
	}
}

func (s *Server) isAuthenticated(session *ClientSession) (string, bool) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for systemID, clientSession := range s.Clients {
		if clientSession == session {
			return systemID, true
		}
	}
	return "", false
}

func (s *Server) handleEnquireLink(session *ClientSession, pdu Pdu) {
	enquireLink := pdu.(*EnquireLink)
	resp, _ := session.EnquireLinkResp(enquireLink.GetHeader().Sequence)
	err := session.Write(resp)
	if err != nil {
		return
	}
}

func (s *Server) handleUnbind(session *ClientSession, pdu Pdu) {
	unbind := pdu.(*Unbind)
	resp, _ := session.UnbindResp(unbind.GetHeader().Sequence)
	err := session.Write(resp)
	if err != nil {
		return
	}
	session.bound = false
}

func (s *Server) handleUnknownPDU(session *ClientSession, pdu Pdu) {
	resp, _ := session.GenericNack(pdu.GetHeader().Sequence, ESME_RINVCMDID)
	err := session.Write(resp)
	if err != nil {
		return
	}
}

func (s *Server) addClient(systemID string, session *ClientSession) {
	logrus.Infof("adding client %s %s", systemID, session.conn.RemoteAddr().String())

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	s.Clients[systemID] = session
	s.clientAddresses[session.conn.RemoteAddr().String()] = systemID
}

func (s *Server) removeClient(session *ClientSession) {
	logrus.Infof("removing client %s %s", session.systemID, session.conn.RemoteAddr().String())

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.Clients, session.systemID)
	delete(s.clientAddresses, session.systemID)
}

func (s *Server) Stop() error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for _, session := range s.Clients {
		session.Close()
	}
	close(s.IncomingChannel)
	close(s.OutgoingChannel)
	return s.listener.Close()
}

func generateMessageID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String())
}
