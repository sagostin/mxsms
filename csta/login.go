package csta

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

// Предопределенные значения платформы и версии, используемые при логине.
var (
	DefaultPlatform = "iPhone" // название платформы, используемой для логина по умолчанию.
	DefaultVersion  = "N/A"    // версия платформы
)

// Login описывает информацию для авторизации на MX-сервере.
type Login struct {
	Type     string `yaml:",omitempty"` // тип учетной записи: User, Server, Group
	User     string // имя пользователя
	Password string // пароль
	Clear    bool   `yaml:",omitempty"` // отсылать пароль в открытом виде
	Platform string `yaml:",omitempty"` // название платформы
	Version  string `yaml:",omitempty"` // версия программного обеспечения
}

// loginRequest возвращает инициализированную команду для авторизации.
func (l *Login) loginRequest() *loginRequest {
	loginType := l.Type // тип логина
	if loginType == "" {
		loginType = "User"
	}
	platform := l.Platform // платформа
	if platform == "" {
		platform = DefaultPlatform
	}
	version := l.Version // версия
	if version == "" {
		version = DefaultVersion
	}
	password := l.Password // пароль
	if !l.Clear {          // пароль передается в зашифрованном виде
		var pwdHash = sha1.Sum([]byte(password))                        // хешируем пароль
		password = base64.StdEncoding.EncodeToString(pwdHash[:]) + "\n" // переводим в base64
	}
	// отсылаем команду логина на сервер
	return &loginRequest{
		Type:     loginType, // тип логина
		Platform: platform,  // платформа
		Version:  version,   // версия платформы
		UserName: l.User,    // имя пользователя
		Password: password,  // пароль
	}
}

// loginRequest описывает запрос авторизации.
type loginRequest struct {
	XMLName  xml.Name `xml:"loginRequest"`
	Type     string   `xml:"type,attr,omitempty"`
	Platform string   `xml:"platform,attr,omitempty"`
	Version  string   `xml:"version,attr,omitempty"`
	UserName string   `xml:"userName"`
	Password string   `xml:"pwd"`
}

// LoginResponce описывает ответ на логин (события loginResponce и loginFailed)
type LoginResponce struct {
	Code       int    `xml:"Code,attr"`       // код ошибки
	SN         string `xml:"sn,attr"`         // серийный номер
	APIVersion int    `xml:"apiversion,attr"` // версия API
	Ext        string `xml:"ext,attr"`        // внутренний телефонный номер пользователя
	JID        string `xml:"userId,attr"`     // внутренний уникальный идентификатор пользователя
	Message    string `xml:",chardata"`       // текст сообщения
}

// Error возвращает текстовое описание ошибки авторизации.
func (l *LoginResponce) Error() string {
	return fmt.Sprintf("login error [%d]: %s", l.Code, l.Message)
}
