package email

import (
	"errors"
	"net/smtp"
	"strings"
)

type loginAuth struct {
	username, password string
}

/*
	Returns an Auth interface implementing the LOGIN authentication mechanism as defined in RFC 4616.

	NOTE: method used by Office 365 circa 2020.

	NOTE: pieced together from:

		- https://github.com/go-gomail/gomail/issues/16#issuecomment-73672398
		- https://github.com/golang/go/issues/9899
		- https://gist.github.com/homme/22b457eb054a07e7b2fb
*/
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) (toServer []byte, E error) {

	if !more {
		return
	}

	command := string(fromServer)
	command = strings.TrimSpace(command)
	command = strings.TrimSuffix(command, ":")
	command = strings.ToLower(command)

	switch command {
	case "username":
		toServer = []byte(a.username)
	case "password":
		toServer = []byte(a.password)
	default:
		// We've already sent everything.
		E = errors.New("unexpected server challenge: " + command)
	}

	return
}
