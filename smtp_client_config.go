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

type SMTPClientMode uint8

const (
	ModeUNENCRYPTED SMTPClientMode = 0
	ModeSTARTTLS    SMTPClientMode = 1
	ModeFORCETLS    SMTPClientMode = 2
)

func (MODE *SMTPClientMode) UnmarshalJSON(V []byte) (E error) {

	var STR string

	E = json.Unmarshal(V, &STR)
	if E != nil {
		return
	}

	STR = strings.ToUpper(strings.TrimSpace(STR))

	switch STR {

	case "UNENCRYPTED":
		*MODE = ModeUNENCRYPTED
	case "STARTTLS":
		*MODE = ModeSTARTTLS
	case "FORCETLS":
		*MODE = ModeFORCETLS
	default:
		E = ErrInvalidSMTPMode
	}

	return
}

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
func (CFG SMTPClientConfig) SimpleSend(MSG ...*Email) (E error) {

	iAuth := LoginAuth(CFG.Username, CFG.Password)
	pTLSCfg := TLSConfig(CFG.Server)
	DIAL_ADDR := fmt.Sprintf("%s:%d", CFG.Server, CFG.Port)

	if len(CFG.Proto) == 0 {
		CFG.Proto = "tcp"
	}

	// FOR DIAL/IO TIMEOUTS
	TIMEOUT_DURATION := time.Millisecond * time.Duration(CFG.TimeoutMsec)
	pDialer := &net.Dialer{
		Timeout:   TIMEOUT_DURATION,
		KeepAlive: -1, // disabled
	}

	var iConn net.Conn
	var pTLSCfgClient *tls.Config = nil

	switch CFG.Mode {

	case ModeSTARTTLS:

		// [1]: open an unencrypted network connection
		iConn, E = pDialer.Dial(CFG.Proto, DIAL_ADDR)
		if E != nil {
			return
		}

		// negotiate TLS in SMTP session
		pTLSCfgClient = pTLSCfg

	case ModeFORCETLS:

		// [1]: open a TLS-secured network connection
		iConn, E = tls.DialWithDialer(pDialer, CFG.Proto, DIAL_ADDR, pTLSCfg)
		if E != nil {
			return
		}

	case ModeUNENCRYPTED:

		// [1]: open an unencrypted network connection
		iConn, E = pDialer.Dial(CFG.Proto, DIAL_ADDR)
		if E != nil {
			return
		}

	default:

		E = ErrInvalidSMTPMode
		return
	}

	var fnTextprotoCreate CreateTextprotoConnFn = nil

	// SMTP SESSION LOGGING
	if len(CFG.SMTPLog) > 0 {

		var FILE *os.File

		if CFG.SMTPLog == "-" {
			FILE = os.Stdout
		} else {
			FILE, E = os.OpenFile(CFG.SMTPLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0660)
			if E != nil {
				return
			}
		}

		fnTextprotoCreate = TextprotoLogged(log.New(FILE, "", log.Ltime|log.Lmicroseconds), true)
	}

	// COMMS TIMEOUT
	if TIMEOUT_DURATION > 0 {
		E = iConn.SetDeadline(time.Now().Add(TIMEOUT_DURATION))
		if E != nil {
			return
		}
	}

	// [2]: ESTABLISH SMTP SESSION
	SESS, E := NewClient(iConn, iAuth, CFG.Server, pTLSCfgClient, fnTextprotoCreate)
	if E != nil {
		return
	}

	// [3]: SEND MESSAGE(S)
	for ix := range MSG {

		// COMMS TIMEOUT
		if TIMEOUT_DURATION > 0 {
			E = iConn.SetDeadline(time.Now().Add(TIMEOUT_DURATION))
			if E != nil {
				return
			}
		}

		E = SESS.Send(MSG[ix])
		if E != nil {
			return
		}
	}

	// [4]: CLOSE SMTP SESSION WHEN FINISHED SENDING
	// NOTE: this also closes the underlying network connection
	E = SESS.Quit()
	return
}
