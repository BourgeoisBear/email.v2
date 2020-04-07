// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
)

/*
	Implements the Simple Mail Transfer Protocol as defined in RFC 5321.
	It also implements the following extensions:
		8BITMIME  RFC 1652
		AUTH      RFC 2554
		STARTTLS  RFC 3207
	Additional extensions may be handled by clients.
*/
type Client struct {
	// This is the TextProtoConn interface used by the Client.
	// It is exported to allow for clients to add extensions.
	Text TextProtoConn

	// cached wrapper function to preserve wrapping on STARTTLS upgrade
	fnNewTextproto CreateTextprotoConnFn

	// keep a reference to the connection so it can be used to create a TLS
	// connection later
	conn net.Conn

	serverName string

	// map of supported extensions
	ext map[string]string

	// supported auth mechanisms
	auth       []string

	localName  string // the name to use in HELO/EHLO
	didHello   bool   // whether we've said HELO/EHLO
	helloError error  // the error from the hello
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.Text.Close()
}

// hello runs a hello exchange if needed.
func (c *Client) hello() error {
	if !c.didHello {
		c.didHello = true
		err := c.ehlo()
		if err != nil {
			c.helloError = c.helo()
		}
	}
	return c.helloError
}

// Hello sends a HELO or EHLO to the server as the given host name.
// Calling this method is only necessary if the client needs control
// over the host name used. The client will introduce itself as "localhost"
// automatically otherwise. If Hello is called, it must be called before
// any of the other methods.
func (c *Client) Hello(localName string) error {
	if err := validateLine(localName); err != nil {
		return err
	}
	if c.didHello {
		return errors.New("smtp: Hello called after other methods")
	}
	c.localName = localName
	return c.hello()
}

// cmd is a convenience function that sends a command and returns the response
func (c *Client) cmd(expectCode int, format string, args ...interface{}) (int, string, error) {
	id, err := c.Text.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}
	c.Text.StartResponse(id)
	defer c.Text.EndResponse(id)
	code, msg, err := c.Text.ReadResponse(expectCode)
	return code, msg, err
}

// helo sends the HELO greeting to the server. It should be used only when the
// server does not support ehlo.
func (c *Client) helo() error {
	c.ext = nil
	_, _, err := c.cmd(250, "HELO %s", c.localName)
	return err
}

// ehlo sends the EHLO (extended hello) greeting to the server. It
// should be the preferred greeting for servers that support it.
func (c *Client) ehlo() error {
	_, msg, err := c.cmd(250, "EHLO %s", c.localName)
	if err != nil {
		return err
	}
	ext := make(map[string]string)
	extList := strings.Split(msg, "\n")
	if len(extList) > 1 {
		extList = extList[1:]
		for _, line := range extList {
			args := strings.SplitN(line, " ", 2)
			if len(args) > 1 {
				ext[args[0]] = args[1]
			} else {
				ext[args[0]] = ""
			}
		}
	}
	if mechs, ok := ext["AUTH"]; ok {
		c.auth = strings.Split(mechs, " ")
	}
	c.ext = ext
	return err
}

// StartTLS sends the STARTTLS command and encrypts all further communication.
// Only servers that advertise the STARTTLS extension support this function.
func (c *Client) StartTLS(config *tls.Config) error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(220, "STARTTLS")
	if err != nil {
		return err
	}
	c.conn = tls.Client(c.conn, config)
	c.Text = c.fnNewTextproto(c.conn)
	return c.ehlo()
}

// Auth authenticates a client using the provided authentication mechanism.
// A failed authentication closes the connection.
// Only servers that advertise the AUTH extension support this function.
func (c *Client) Auth(a Auth) error {
	if err := c.hello(); err != nil {
		return err
	}
	encoding := base64.StdEncoding
	mech, resp, err := a.Start(&ServerInfo{c.serverName, c.IsTLS(), c.auth})
	if err != nil {
		c.Quit()
		return err
	}
	resp64 := make([]byte, encoding.EncodedLen(len(resp)))
	encoding.Encode(resp64, resp)
	code, msg64, err := c.cmd(0, strings.TrimSpace(fmt.Sprintf("AUTH %s %s", mech, resp64)))
	for err == nil {
		var msg []byte
		switch code {
		case 334:
			msg, err = encoding.DecodeString(msg64)
		case 235:
			// the last message isn't base64 because it isn't a challenge
			msg = []byte(msg64)
		default:
			err = &textproto.Error{Code: code, Msg: msg64}
		}
		if err == nil {
			resp, err = a.Next(msg, code == 334)
		}
		if err != nil {
			// abort the AUTH
			c.cmd(501, "*")
			c.Quit()
			break
		}
		if resp == nil {
			break
		}
		resp64 = make([]byte, encoding.EncodedLen(len(resp)))
		encoding.Encode(resp64, resp)
		code, msg64, err = c.cmd(0, string(resp64))
	}
	return err
}

// Mail issues a MAIL command to the server using the provided email address.
// If the server supports the 8BITMIME extension, Mail adds the BODY=8BITMIME
// parameter.
// This initiates a mail transaction and is followed by one or more Rcpt calls.
func (c *Client) Mail(from string) error {
	if err := validateLine(from); err != nil {
		return err
	}
	if err := c.hello(); err != nil {
		return err
	}
	cmdStr := "MAIL FROM:<%s>"
	if c.ext != nil {
		if _, ok := c.ext["8BITMIME"]; ok {
			cmdStr += " BODY=8BITMIME"
		}
	}
	_, _, err := c.cmd(250, cmdStr, from)
	return err
}

// Rcpt issues a RCPT command to the server using the provided email address.
// A call to Rcpt must be preceded by a call to Mail and may be followed by
// a Data call or another Rcpt call.
func (c *Client) Rcpt(to string) error {
	if err := validateLine(to); err != nil {
		return err
	}
	_, _, err := c.cmd(25, "RCPT TO:<%s>", to)
	return err
}

type dataCloser struct {
	c *Client
	io.WriteCloser
}

func (d *dataCloser) Close() error {
	d.WriteCloser.Close()
	_, _, err := d.c.Text.ReadResponse(250)
	return err
}

// Data issues a DATA command to the server and returns a writer that
// can be used to write the mail headers and body. The caller should
// close the writer before calling any more methods on c. A call to
// Data must be preceded by one or more calls to Rcpt.
func (c *Client) Data() (io.WriteCloser, error) {
	_, _, err := c.cmd(354, "DATA")
	if err != nil {
		return nil, err
	}
	return &dataCloser{c, c.Text.DotWriter()}, nil
}

var testHookStartTLS func(*tls.Config) // nil, except for tests

// Extension reports whether an extension is support by the server.
// The extension name is case-insensitive. If the extension is supported,
// Extension also returns a string that contains any parameters the
// server specifies for the extension.
func (c *Client) Extension(ext string) (bool, string) {
	if err := c.hello(); err != nil {
		return false, ""
	}
	if c.ext == nil {
		return false, ""
	}
	ext = strings.ToUpper(ext)
	param, ok := c.ext[ext]
	return ok, param
}

// Reset sends the RSET command to the server, aborting the current mail
// transaction.
func (c *Client) Reset() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(250, "RSET")
	return err
}

// Noop sends the NOOP command to the server. It does nothing but check
// that the connection to the server is okay.
func (c *Client) Noop() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(250, "NOOP")
	return err
}

// Quit sends the QUIT command and closes the connection to the server.
func (c *Client) Quit() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(221, "QUIT")
	if err != nil {
		return err
	}
	return c.Text.Close()
}

// validateLine checks to see if a line has CR or LF as per RFC 5321
func validateLine(line string) error {
	if strings.ContainsAny(line, "\n\r") {
		return errors.New("smtp: A line must not contain CR or LF")
	}
	return nil
}

// TLS config recommendations per "So you want to expose Go on the Internet":
// https://blog.cloudflare.com/exposing-go-on-the-internet/
func TLSConfig(hostName string) *tls.Config {

	return &tls.Config{

		ServerName: hostName,

		// Causes servers to use Go's default ciphersuite preferences,
		// which are tuned to avoid attacks. Does nothing on clients.
		PreferServerCipherSuites: true,

		// Only use curves which have assembly implementations
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519, // Go 1.8 only
		},

		MinVersion: tls.VersionTLS12,

		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Go 1.8 only
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Go 1.8 only
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

// Send an e-mail using the established SMTP session.
func (c *Client) Send(e *Email) (E error) {

	// PARSE/VERIFY ADDRESSES
	to, E := e.ParseToFromAddrs()
	if E != nil {
		return
	}

	sender, E := e.ParseSender()
	if E != nil {
		return
	}

	// MESSAGE-TO-BYTESTREAM
	raw, E := e.Bytes()
	if E != nil {
		return
	}

	// CMD: SENDER & RECIPIENTS
	E = c.Mail(sender.Address)
	if E != nil {
		return
	}

	for _, addrRecip := range to {
		E = c.Rcpt(addrRecip.Address)
		if E != nil {
			return
		}
	}

	// CMD: DATA
	w, E := c.Data()
	if E != nil {
		return
	}

	// WRITE DATA BYTES TO SERVER
	_, E = w.Write(raw)
	if E != nil {
		w.Close()
	} else {
		E = w.Close()
	}

	return
}

func (c *Client) IsTLS() bool {

	if c.conn != nil {
		_, bIsTLS := c.conn.(*tls.Conn)
		return bIsTLS
	}

	return false
}

/*
	NewClient creates an SMTP Client from an existing connection and
	a server hostname, then attempts to establish an active SMTP session
	with it.

	Attempts STARTTLS negotiation if `pSTARTTLSCfg` is provided, and the server provides the
	STARTTLS extension.  Skips STARTTLS negotiation if nil.

	`fnNewTextproto` creates a TextProtoConn interface from a net.Conn interface.
	This allows us to inject textproto.Conn wrappers that insert/remove/capture
	SMTP traffic before it hits the wire.

	If fnNewTextproto is left nil, the underlying textproto will come from
	textproto.NewConn().

	Close with .Quit() method to end session.
*/
func NewClient(
	iConn           net.Conn,
	iAuth           Auth,
	serverName      string,
	pSTARTTLSCfg    *tls.Config,
	fnNewTextproto  CreateTextprotoConnFn,
) (c *Client, E error) {

	if fnNewTextproto == nil {
		fnNewTextproto = textprotoFromConn
	}

	iTextproto := fnNewTextproto(iConn)
	_, _, E = iTextproto.ReadResponse(220)
	if E != nil {
		iTextproto.Close()
		return
	}

	c = &Client{
		Text:           iTextproto,
		fnNewTextproto: fnNewTextproto,
		conn:           iConn,
		serverName:     serverName,
		localName:      "localhost",
	}

	defer func() {
		if E != nil {
			c.Close()
		}
	}()

	E = c.Hello("localhost")
	if E != nil {
		return
	}

	// NEGOTIATE STARTTLS IF REQUESTED & CONNECTION IS UNENCRYPTED
	if !c.IsTLS() && (pSTARTTLSCfg != nil) {

		if bHasExt, _ := c.Extension("STARTTLS"); bHasExt {
			E = c.StartTLS(pSTARTTLSCfg)
			if E != nil {
				return
			}
		} else {
			E = ErrSTARTTLSNotOffered
			return
		}
	}

	if iAuth != nil {

		if bHasExt, _ := c.Extension("AUTH"); bHasExt {
			E = c.Auth(iAuth)
			if E != nil {
				return
			}
		}
	}

	return
}
