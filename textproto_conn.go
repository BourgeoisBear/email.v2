package email

import (
	"io"
	"net"
	"net/textproto"
)

/*
TextProtoConn interface implements all textproto actions used by the SMTP Client.

Communication hits textproto before crypto and the wire, so this is useful
for inserting/removing/capturing commands before they are encrypted and
sent down the wire.
*/
type TextProtoConn interface {
	StartResponse(id uint)
	EndResponse(id uint)

	Cmd(format string, args ...interface{}) (id uint, err error)
	ReadResponse(expectCode int) (code int, message string, err error)
	DotWriter() io.WriteCloser

	Close() error
}

// CreateTextprotoConnFn is a wrapper for intercepting net.Conn traffic
type CreateTextprotoConnFn func(net.Conn) TextProtoConn

func textprotoFromConn(iConn net.Conn) TextProtoConn {
	return textproto.NewConn(iConn)
}
