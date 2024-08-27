package csta

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

// Predefined platform and version values used for login.
var (
	DefaultPlatform = "iPhone" // default platform name used for login
	DefaultVersion  = "N/A"    // platform version
)

// Login describes the information for authorization on the MX server.
type Login struct {
	Type     string `yaml:",omitempty"` // account type: User, Server, Group
	User     string // username
	Password string // password
	Clear    bool   `yaml:",omitempty"` // send password in clear text
	Platform string `yaml:",omitempty"` // platform name
	Version  string `yaml:",omitempty"` // software version
}

// loginRequest returns an initialized command for authorization.
func (l *Login) loginRequest() *loginRequest {
	loginType := l.Type // login type
	if loginType == "" {
		loginType = "User"
	}
	platform := l.Platform // platform
	if platform == "" {
		platform = DefaultPlatform
	}
	version := l.Version // version
	if version == "" {
		version = DefaultVersion
	}
	password := l.Password // password
	if !l.Clear {          // password is transmitted in encrypted form
		var pwdHash = sha1.Sum([]byte(password))                        // hash the password
		password = base64.StdEncoding.EncodeToString(pwdHash[:]) + "\n" // convert to base64
	}
	// send login command to the server
	return &loginRequest{
		Type:     loginType, // login type
		Platform: platform,  // platform
		Version:  version,   // platform version
		UserName: l.User,    // username
		Password: password,  // password
	}
}

// loginRequest describes the authorization request.
type loginRequest struct {
	XMLName  xml.Name `xml:"loginRequest"`
	Type     string   `xml:"type,attr,omitempty"`
	Platform string   `xml:"platform,attr,omitempty"`
	Version  string   `xml:"version,attr,omitempty"`
	UserName string   `xml:"userName"`
	Password string   `xml:"pwd"`
}

// LoginResponse describes the response to login (loginResponse and loginFailed events)
type LoginResponse struct {
	Code       int    `xml:"Code,attr"`       // error code
	SN         string `xml:"sn,attr"`         // serial number
	APIVersion int    `xml:"apiversion,attr"` // API version
	Ext        string `xml:"ext,attr"`        // user's internal phone number
	JID        string `xml:"userId,attr"`     // user's internal unique identifier
	Message    string `xml:",chardata"`       // message text
}

// Error returns a text description of the authorization error.
func (l *LoginResponse) Error() string {
	return fmt.Sprintf("login error [%d]: %s", l.Code, l.Message)
}
