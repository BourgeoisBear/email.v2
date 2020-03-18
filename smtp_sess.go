package email

import (
	"crypto/tls"
	"errors"
	"net"
	"net/smtp"
)

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

// Represents an SMTP client with an *established* SMTP session.
type EstablishedSession struct {
	*smtp.Client
}

/*
	Establishes an SMTP session using a given network connection and authentication interface.

	Close with .Quit() method to end session.

	Attempts STARTTLS negotiation if pSTARTTLSCfg is provided and server provides the extension.  Skips if nil.
*/
func NewSession(
	iConn net.Conn,
	iAuth smtp.Auth,
	serverName string,
	pSTARTTLSCfg *tls.Config,
) (CLI EstablishedSession, E error) {

	CLI.Client, E = smtp.NewClient(iConn, serverName)
	if E != nil {
		return
	}

	defer func() {
		if E != nil {
			CLI.Close()
		}
	}()

	E = CLI.Hello("localhost")
	if E != nil {
		return
	}

	// TODO: RENAME PACKAGE
	// TODO: UNIT TESTS
	// TODO: LOGGING?

	// NEGOTIATE STARTTLS IF REQUESTED
	if pSTARTTLSCfg != nil {

		if bHasExt, _ := CLI.Extension("STARTTLS"); bHasExt {
			E = CLI.StartTLS(pSTARTTLSCfg)
			if E != nil {
				return
			}
		} else {
			E = ErrSTARTTLSNotOffered
			return
		}
	}

	if iAuth != nil {
		if ok, _ := CLI.Extension("AUTH"); ok {
			E = CLI.Auth(iAuth)
			if E != nil {
				return
			}
		}
	}

	return
}

// Send an e-mail using the established SMTP session.
func (CLI EstablishedSession) Send(MSG Email) (E error) {

	if CLI.Client == nil {
		return errors.New("nil SMTP client")
	}

	// PARSE/VERIFY ADDRESSES
	to, E := MSG.ParseToFromAddrs()
	if E != nil {
		return
	}

	sender, E := MSG.ParseSender()
	if E != nil {
		return
	}

	// MESSAGE-TO-BYTESTREAM
	raw, E := MSG.Bytes()
	if E != nil {
		return
	}

	// SEND VIA CLIENT
	E = CLI.Mail(sender.Address)
	if E != nil {
		return
	}

	for _, addrRecip := range to {
		E = CLI.Rcpt(addrRecip.Address)
		if E != nil {
			return
		}
	}

	w, E := CLI.Data()
	if E != nil {
		return
	}

	_, E = w.Write(raw)
	if E != nil {
		w.Close()
	} else {
		E = w.Close()
	}

	return
}
