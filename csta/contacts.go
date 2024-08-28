package csta

import "encoding/xml"

// Contact описывает информацию о пользователе из адресной книги.
type Contact struct {
	JID        JID    `xml:"jid,attr" json:"jid,string"`
	Presence   string `xml:"presence,attr" json:"status,omitempty"`
	Note       string `xml:"presenceNote,attr" json:"note,omitempty"`
	FirstName  string `xml:"firstName" json:"firstName"`
	LastName   string `xml:"lastName" json:"lastName"`
	Ext        string `xml:"businessPhone" json:"ext"`
	HomePhone  string `xml:"homePhone" json:"homePhone,omitempty"`
	CellPhone  string `xml:"cellPhone" json:"cellPhone,omitempty"`
	Email      string `xml:"email" json:"email,omitempty"`
	HomeSystem JID    `xml:"homeSystem" json:"homeSystem,string,omitempty"`
	DID        string `xml:"did" json:"did,omitempty"`
	ExchangeID string `xml:"exchangeId" json:"exchangeId,omitempty"`
}

// Addressbook возвращает адресную книгу.
func (c *Conn) Addressbook() ([]*Contact, error) {
	// команда для запроса адресной книги
	var cmdGetAddressBook = &struct {
		XMLName xml.Name `xml:"iq"`
		Type    string   `xml:"type,attr"`
		ID      string   `xml:"id,attr"`
		Index   uint     `xml:"index,attr"`
	}{Type: "get", ID: "addressbook"}
	// отправляем запрос
	if err := c.Send(cmdGetAddressBook); err != nil {
		return nil, err
	}
	var contacts []*Contact // адресная книга
	//  инициализируем обработку ответов сервера
	if err := c.HandleWait(func(resp *Response) error {
		// разбираем полученный кусок адресной книги
		var abList = new(struct {
			Size     uint       `xml:"size,attr" json:"size"`
			Index    uint       `xml:"index,attr" json:"index,omitempty"`
			Contacts []*Contact `xml:"abentry" json:"contacts,omitempty"`
		})
		if err := resp.Decode(abList); err != nil {
			return err
		}
		// инициализируем адресную книгу, если она еще не была инициализирована
		if contacts == nil {
			contacts = make([]*Contact, 0, abList.Size)
		}
		// заполняем адресную книгу полученными контактами
		contacts = append(contacts, abList.Contacts...)
		// проверяем, что получена вся адресная книга
		if (abList.Index+1)*50 < abList.Size {
			// увеличиваем номер для получения следующей "страницы" контактов
			cmdGetAddressBook.Index = abList.Index + 1
			// отправляем запрос на получение следующей порции
			return c.Send(cmdGetAddressBook)
		}
		return Stop // заканчиваем обработку, т.к. все получили
	}, ReadTimeout, "ablist"); err != nil {
		return nil, err
	}
	return contacts, nil
}
