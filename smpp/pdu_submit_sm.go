package smpp

import (
	"bytes"
)

var (
	// Required SubmitSm Fields
	reqSSMFields = []string{
		SERVICE_TYPE,
		SOURCE_ADDR_TON,
		SOURCE_ADDR_NPI,
		SOURCE_ADDR,
		DEST_ADDR_TON,
		DEST_ADDR_NPI,
		DESTINATION_ADDR,
		ESM_CLASS,
		PROTOCOL_ID,
		PRIORITY_FLAG,
		SCHEDULE_DELIVERY_TIME,
		VALIDITY_PERIOD,
		REGISTERED_DELIVERY,
		REPLACE_IF_PRESENT_FLAG,
		DATA_CODING,
		SM_DEFAULT_MSG_ID,
		SM_LENGTH,
		SHORT_MESSAGE,
	}
)

type SubmitSm struct {
	*Header
	mandatoryFields map[string]Field
	tlvFields       map[uint16]*TLVField
}

func NewSubmitSm(hdr *Header, b []byte) (*SubmitSm, error) {
	r := bytes.NewBuffer(b)
	fields, tlvs, err := create_pdu_fields(reqSSMFields, r)
	if err != nil {
		return nil, err
	}
	s := &SubmitSm{hdr, fields, tlvs}
	return s, nil
}

func (s *SubmitSm) GetField(f string) Field {
	return s.mandatoryFields[f]
}

func (s *SubmitSm) Fields() map[string]Field {
	return s.mandatoryFields
}

func (s *SubmitSm) MandatoryFieldsList() []string {
	return reqSSMFields
}

func (s *SubmitSm) Ok() bool {
	return true
}

func (s *SubmitSm) GetHeader() *Header {
	return s.Header
}

func (s *SubmitSm) SetField(f string, v interface{}) error {
	if s.validate_field(f, v) {
		field := NewField(f, v)
		if field != nil {
			s.mandatoryFields[f] = field
			return nil
		}
	}
	return FieldValueErr
}

func (s *SubmitSm) SetSeqNum(i uint32) {
	s.Header.Sequence = i
}

func (s *SubmitSm) SetTLVField(t, l int, v []byte) error {
	if l != len(v) {
		return TLVFieldLenErr
	}
	s.tlvFields[uint16(t)] = &TLVField{uint16(t), uint16(l), v}
	return nil
}

func (s *SubmitSm) validate_field(f string, v interface{}) bool {
	return included_check(s.MandatoryFieldsList(), f) && validate_pdu_field(f, v)
}

func (s *SubmitSm) TLVFields() map[uint16]*TLVField {
	return s.tlvFields
}

func (s *SubmitSm) writeFields() []byte {
	b := []byte{}
	for _, i := range s.MandatoryFieldsList() {
		v := s.mandatoryFields[i].ByteArray()
		b = append(b, v...)
	}
	return b
}

func (s *SubmitSm) writeTLVFields() []byte {
	b := []byte{}
	for _, v := range s.tlvFields {
		b = append(b, v.Writer()...)
	}

	return b
}

func (s *SubmitSm) Writer() []byte {
	// Set SM_LENGTH
	sm := len(s.GetField(SHORT_MESSAGE).ByteArray())
	s.SetField(SM_LENGTH, sm)

	// Safely get field data
	fields := s.writeFields()
	if fields == nil {
		fields = []byte{}
	}

	// Safely get TLV field data
	tlvFields := s.writeTLVFields()
	if tlvFields == nil {
		tlvFields = []byte{}
	}

	// Combine fields and TLV fields
	b := append(fields, tlvFields...)

	// Build the header
	h := packUi32(uint32(len(b) + 16))
	h = append(h, packUi32(uint32(SUBMIT_SM))...)
	h = append(h, packUi32(uint32(s.Header.Status))...)
	h = append(h, packUi32(s.Header.Sequence)...)

	// Return the combined header and body
	return append(h, b...)
}
