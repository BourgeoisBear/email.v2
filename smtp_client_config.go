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
	KeepAlive   net.KeepAliveConfig
}

// Dial to an SMTP server & establish an SMTP session per settings
// in SMTPClientConfig.
func (cfg SMTPClientConfig) Dial() (*Client, error) {

	iAuth := LoginAuth(cfg.Username, cfg.Password)
	pTLSCfg := TLSConfig(cfg.Server)
	dialAddr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)

	if len(cfg.Proto) == 0 {
		cfg.Proto = "tcp"
	}

	// FOR DIAL/IO TIMEOUTS
	dTimeout := time.Millisecond * time.Duration(cfg.TimeoutMsec)
	pDialer := &net.Dialer{
		Timeout:         dTimeout,
		KeepAlive:       -1,
		KeepAliveConfig: cfg.KeepAlive,
	}

	var iConn net.Conn
	var pTLSCfgClient *tls.Config
	var err error

	switch cfg.Mode {

	case ModeSTARTTLS:

		// [1]: open an unencrypted network connection
		if iConn, err = pDialer.Dial(cfg.Proto, dialAddr); err != nil {
			return nil, err
		}

		// negotiate TLS in SMTP session
		pTLSCfgClient = pTLSCfg

	case ModeFORCETLS:

		// [1]: open a TLS-secured network connection
		if iConn, err = tls.DialWithDialer(pDialer, cfg.Proto, dialAddr, pTLSCfg); err != nil {
			return nil, err
		}

	case ModeUNENCRYPTED:

		// [1]: open an unencrypted network connection
		if iConn, err = pDialer.Dial(cfg.Proto, dialAddr); err != nil {
			return nil, err
		}

	default:

		return nil, ErrInvalidSMTPMode
	}

	var fnTextprotoCreate CreateTextprotoConnFn

	// SMTP SESSION LOGGING
	if len(cfg.SMTPLog) > 0 {

		var pF *os.File

		if cfg.SMTPLog == "-" {
			pF = os.Stdout
		} else {
			pF, err = os.OpenFile(cfg.SMTPLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0660)
			if err != nil {
				return nil, err
			}
		}

		fnTextprotoCreate = TextprotoLogged(log.New(pF, "", log.Ltime|log.Lmicroseconds), true)
	}

	// COMMS TIMEOUT
	if dTimeout > 0 {
		if err = iConn.SetDeadline(time.Now().Add(dTimeout)); err != nil {
			return nil, err
		}
	}

	// ESTABLISH SMTP SESSION
	pCli, err := NewClient(iConn, iAuth, cfg.Server, pTLSCfgClient, fnTextprotoCreate)
	if err != nil {
		return nil, err
	}
	pCli.TimeoutMsec = cfg.TimeoutMsec
	return pCli, nil
}

/*
SimpleSend is a way to quickly connect to an SMTP server, send multiple
messages, then disconnect.

Example

	cfg := SMTPClientConfig{
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

	E := cfg.SimpleSend(oEmail)
	if E != nil { return E }
*/
func (cfg SMTPClientConfig) SimpleSend(sMsgs ...*Email) error {

	pCli, err := cfg.Dial()
	if err != nil {
		return err
	}

	for ix := range sMsgs {
		if err = pCli.Send(sMsgs[ix]); err != nil {
			return err
		}
	}

	// CLOSE SMTP SESSION WHEN FINISHED SENDING
	// NOTE: this also closes the underlying network connection
	return pCli.Quit()
}
