package email

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"crypto/tls"
	"net"

	"log"
	"os"
)

// SMTPClientMode is the encryption mode to be used by the SMTP client
type SMTPClientMode uint8

const (
	// ModeUNENCRYPTED = no encryption
	ModeUNENCRYPTED SMTPClientMode = iota
	// ModeSTARTTLS = STARTTLS negotiation
	ModeSTARTTLS
	// ModeFORCETLS = FORCED TLS
	ModeFORCETLS
)

// UnmarshalJSON decodes a string into an SMTPClientConfig value.
func (mode *SMTPClientMode) UnmarshalJSON(val []byte) error {

	var szStr string
	E := json.Unmarshal(val, &szStr)
	if E != nil {
		return E
	}

	szStr = strings.ToUpper(strings.TrimSpace(szStr))
	switch szStr {
	case "UNENCRYPTED":
		*mode = ModeUNENCRYPTED
	case "STARTTLS":
		*mode = ModeSTARTTLS
	case "FORCETLS":
		*mode = ModeFORCETLS
	default:
		return ErrInvalidSMTPMode
	}
	return nil
}

// SMTPClientConfig holds parameters for connecting to an SMTP server.
type SMTPClientConfig struct {
	Server      string
	Port        uint16
	Username    string
	Password    string
	Mode        SMTPClientMode
	TimeoutMsec uint32
	Proto       string // dial protocol: `tcp`, `tcp4`, or `tcp6`; defaults to `tcp`
	SMTPLog     string // path to SMTP log: complete filepath, "-" for STDOUT, or empty to disable SMTP logging
}

/*
SimpleSend is a way to quickly connect to an SMTP server, send multiple
messages, then disconnect.

Example

	oCfg := SMTPClientConfig{
	  Server:   "mx.test.com",
	  Port:     587,
	  Username: "test@test.com",
	  Password: "...",
	  Mode:     ModeSTARTTLS,
	  // SMTPLog:  "-",  // note: uncomment to log SMTP session to STDOUT
	}

	oEmail := NewEmail()
	oEmail.From    = "test@test.com"
	oEmail.To      = []string{"test_receiver@eggplant.pro"}
	oEmail.Subject = "Test Message"
	oEmail.Text    = []byte("Whoomp there it is!")

	E := oCfg.SimpleSend(oEmail)
	if E != nil { return E }
*/
func (pCfg SMTPClientConfig) SimpleSend(sMsgs ...*Email) error {

	iAuth := LoginAuth(pCfg.Username, pCfg.Password)
	pTLSCfg := TLSConfig(pCfg.Server)
	dialAddr := fmt.Sprintf("%s:%d", pCfg.Server, pCfg.Port)

	if len(pCfg.Proto) == 0 {
		pCfg.Proto = "tcp"
	}

	// FOR DIAL/IO TIMEOUTS
	dTimeout := time.Millisecond * time.Duration(pCfg.TimeoutMsec)
	pDialer := &net.Dialer{
		Timeout:   dTimeout,
		KeepAlive: -1, // disabled
	}

	var iConn net.Conn
	var pTLSCfgClient *tls.Config
	var E error

	switch pCfg.Mode {

	case ModeSTARTTLS:

		// [1]: open an unencrypted network connection
		if iConn, E = pDialer.Dial(pCfg.Proto, dialAddr); E != nil {
			return E
		}

		// negotiate TLS in SMTP session
		pTLSCfgClient = pTLSCfg

	case ModeFORCETLS:

		// [1]: open a TLS-secured network connection
		if iConn, E = tls.DialWithDialer(pDialer, pCfg.Proto, dialAddr, pTLSCfg); E != nil {
			return E
		}

	case ModeUNENCRYPTED:

		// [1]: open an unencrypted network connection
		if iConn, E = pDialer.Dial(pCfg.Proto, dialAddr); E != nil {
			return E
		}

	default:

		return ErrInvalidSMTPMode
	}

	var fnTextprotoCreate CreateTextprotoConnFn

	// SMTP SESSION LOGGING
	if len(pCfg.SMTPLog) > 0 {

		var pF *os.File

		if pCfg.SMTPLog == "-" {
			pF = os.Stdout
		} else {
			pF, E = os.OpenFile(pCfg.SMTPLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0660)
			if E != nil {
				return E
			}
		}

		fnTextprotoCreate = TextprotoLogged(log.New(pF, "", log.Ltime|log.Lmicroseconds), true)
	}

	// COMMS TIMEOUT
	if dTimeout > 0 {
		if E = iConn.SetDeadline(time.Now().Add(dTimeout)); E != nil {
			return E
		}
	}

	// [2]: ESTABLISH SMTP SESSION
	pCli, E := NewClient(iConn, iAuth, pCfg.Server, pTLSCfgClient, fnTextprotoCreate)
	if E != nil {
		return E
	}

	// [3]: SEND MESSAGE(S)
	for ix := range sMsgs {

		// COMMS TIMEOUT
		if dTimeout > 0 {
			if E = iConn.SetDeadline(time.Now().Add(dTimeout)); E != nil {
				return E
			}
		}

		if E = pCli.Send(sMsgs[ix]); E != nil {
			return E
		}
	}

	// [4]: CLOSE SMTP SESSION WHEN FINISHED SENDING
	// NOTE: this also closes the underlying network connection
	return pCli.Quit()
}
