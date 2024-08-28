package csta

import (
	"encoding/xml"
	"fmt"
	"strconv"
)

// Login описывает основные параметры для авторизации пользователя, используемые
// сервером MX.
type Login struct {
	UserName   string `xml:"userName" json:"userName"`
	Password   string `xml:"pwd" json:"password"`
	Type       string `xml:"type,attr,omitempty" json:"type"`
	ClientType string `xml:"clientType,attr,omitempty" json:"clientType,omitempty"`
	ServerType string `xml:"serverType,attr,omitempty" json:"serverType,omitempty"`
	Platform   string `xml:"platform,attr,omitempty" json:"platform,omitempty"`
	Version    string `xml:"version,attr,omitempty" json:"version,omitempty"`
	LoginCapab string `xml:"loginCapab,attr,omitempty" json:"сapab,omitempty"`
	MediaCapab string `xml:"mediaCapab,attr,omitempty" json:"mediaCapab,omitempty"`
	ABNotify   bool   `xml:"abNotify,attr,omitempty" json:"abNotify,omitempty"`
}

// Login отправляет запрос на авторизацию пользователя.
func (c *Conn) Login(login Login) (*Info, error) {
	// хешируем пароль, если он уже не в виде хеша
	var hashed bool             // флаг зашифрованного пароля
	var passwd = login.Password // пароль пользователя для авторизации
	/*// эвристическим способом проверяем, что пароль похож на base64 от sha1.
	if len(passwd) > 4 && passwd[len(passwd)-1] == '\n' {
		data, err := base64.StdEncoding.DecodeString(passwd[:len(passwd)-1])
		hashed = (err == nil && len(data) == sha1.Size)
	}
	// если пароль еще не представлен в виде base64 от sha1, то делаем это
	if !hashed {
		pwdHash := sha1.Sum([]byte(passwd))
		passwd = base64.StdEncoding.EncodeToString(pwdHash[:]) + "\n"
	}*/
	// формируем команду для авторизации пользователя
	cmd := &struct {
		XMLName  xml.Name `xml:"loginRequest"`
		Login             // копируем все параметры логина
		Push     bool     `xml:"push_ntf,attr"`
		Password string   `xml:"pwd"` // заменяем пароль на хеш
	}{Login: login, Password: passwd}
	// отправляем команду и ожидаем ответа
send:
	resp, err := c.SendWithResponse(cmd)
	if err != nil {
		return nil, err
	}
	// разбираем в зависимости от имени ответа
	switch resp.Name {
	case "loginResponce": // пользователь успешно авторизован
		// сохраняем информацию о соединении
		var info Info
		err = resp.Decode(&info)
		// c.mul.Lock()
		// if c.logger != nil {
		// 	var mxid = "MX" + info.SN
		// 	if info.JID != 0 {
		// 		mxid += "-" + info.Ext
		// 	}
		// 	c.logger = c.logger.New(mxid)
		// }
		// c.mul.Unlock()
		c.mu.Lock()
		c.Info = info
		c.mu.Unlock()
		return &info, err
	case "loginFailed": // ошибка авторизации
		var loginError = new(LoginError)
		if err := resp.Decode(loginError); err != nil {
			return nil, err
		}
		// если ошибка связана с тем, что пароль передан в виде хеш,
		// то повторяем попытку авторизации с паролем в открытом виде
		if hashed && loginError.APIVersion > 2 &&
			(loginError.Code == 2 || loginError.Code == 4) {
			hashed = false
			cmd.Password = login.Password
			goto send // повторяем с открытым паролем
		}
		return nil, loginError // возвращаем ошибку авторизации
	default: // неизвестный ответ, который мы не знаем как разбирать
		return nil, fmt.Errorf("unknown login response %s", resp.Name)
	}
}

// Info описывает информацию о сервере и авторизованном пользователе.
type Info struct {
	// уникальный идентификатор сервера  MX
	SN string `xml:"sn,attr" json:"sn,omitempty"`
	// внутренний номер авторизованного пользователя MX
	// может быть пустым в случае серверной авторизации
	Ext string `xml:"ext,attr" json:"ext,omitempty"`
	// уникальный идентификатор пользователя MX
	// может быть 0, в случае серверной авторизации
	JID JID `xml:"userId,attr" json:"jid,string"`
	// пароль для софтфона
	SoftPhonePwd string `xml:"softPhonePwd,attr" json:"softPhonePwd"`
}

// Logout отправляет команду о завершении пользовательской сессии.
func (c *Conn) Logout() error {
	var err = c.Send("<logout/>")
	// // удаляем поля из лога
	// c.mul.Lock()
	// if c.logger != nil {
	// 	c.logger = c.logger.New("mx")
	// }
	// c.mul.Unlock()
	return err
}

// JID описывает формат уникального идентификатора, используемого сервером MX.
type JID = uint64

// ParseJID разбирает строковое представление JID.
func ParseJID(jid string) (JID, error) {
	return strconv.ParseUint(jid, 10, 64)
}
