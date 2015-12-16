package main

import (
	"log"
	"time"

	"github.com/mdigger/smpp"
)

var EnquireLinkDuration = time.Second * 10

type SMPP struct {
	Address  string
	User     string
	Password string
	trx      *smpp.Transceiver
	logger   *log.Logger
}

func (s *SMPP) Connect() error {
	trx, err := smpp.NewTransceiver(s.Address, EnquireLinkDuration, smpp.Params{
		smpp.SYSTEM_TYPE: "SMPP",
		smpp.SYSTEM_ID:   s.User,
		smpp.PASSWORD:    s.Password,
	})
	if err != nil {
		return err
	}
	s.trx = trx
	return nil
}

func (s *SMPP) Close() error {
	return s.trx.Close()
}

func (s *SMPP) Send(from, to, msg string) (seq uint32, err error) {
	return s.trx.SubmitSm(from, to, msg, &smpp.Params{
		smpp.DEST_ADDR_TON: 1,
		smpp.DEST_ADDR_NPI: 1,
	})
}
