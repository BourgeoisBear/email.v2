package email

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
	"unicode/utf8"
)

const (
	ANSI_RESET         = "\u001b[0m"
	ANSI_COLOR_RQ_CMD  = "\u001b[38;5;11m" // yellow
	ANSI_COLOR_RQ_BODY = "\u001b[38;5;14m" // cyan
	ANSI_COLOR_RSP_OK  = "\u001b[38;5;10m" // green
	ANSI_COLOR_RSP_ERR = "\u001b[38;5;9m"  // red

	MODE_SEND = "C>"
	MODE_RECV = "<S"
)

// wrap lines longer than `MaxLineLength`, indents lines after the first by two spaces.
func wrapLong(line string) (parts []string) {

	tmp := []string{}
	nLen := 0

	commit := func() {

		if nLen == 0 {
			return
		}

		var indent string
		if len(parts) > 0 {
			indent = "  "
		}

		parts = append(parts, indent+strings.Join(tmp, " "))
		tmp = []string{}
		nLen = 0
	}

	push := func(s string) {

		wLen := utf8.RuneCountInString(s)

		if (nLen + wLen + 1) > MaxLineLength {
			commit()
		}

		tmp = append(tmp, s)
		nLen += (wLen + 1)
	}

	words := strings.Split(line, " ")
	for _, W := range words {
		push(W)
	}

	commit()

	return
}

// indent every line in `txt` by the `indent` string.
func indentWrap(txt, indent string) string {

	parts := []string{}

	txt = strings.ReplaceAll(txt, "\r", "")
	lines := strings.Split(txt, "\n")

	for _, line := range lines {

		runes := utf8.RuneCountInString(line)

		if runes > MaxLineLength {
			parts = append(parts, wrapLong(line)...)
		} else {
			parts = append(parts, line)
		}
	}

	return indent + strings.Join(parts, "\n"+indent)
}

type smtpLog struct {
	*log.Logger
	Colors bool
}

func (L *smtpLog) log(txt, mode, color string) {

	txt = indentWrap(txt, mode+"\t")

	if L.Colors {
		txt = color + txt + ANSI_RESET
	}

	L.Println("\n" + txt)
}

type loggedWriteCloser struct {
	io.WriteCloser
	smtpLog
}

func (WC loggedWriteCloser) Write(p []byte) (n int, err error) {

	WC.smtpLog.log(string(p), MODE_SEND, ANSI_COLOR_RQ_BODY)
	return WC.WriteCloser.Write(p)
}

type loggedTextProtoConn struct {
	*textproto.Conn
	smtpLog
}

func (TPC loggedTextProtoConn) Cmd(format string, args ...interface{}) (id uint, E error) {

	txt := fmt.Sprintf(format, args...)
	TPC.smtpLog.log(txt, MODE_SEND, ANSI_COLOR_RQ_CMD)

	return TPC.Conn.Cmd(format, args...)
}

func (TPC loggedTextProtoConn) ReadResponse(expectCode int) (code int, message string, E error) {

	code, message, E = TPC.Conn.ReadResponse(expectCode)

	var txt string
	var color string

	if E == nil {
		txt = fmt.Sprintf("%d - %s", code, message)
		color = ANSI_COLOR_RSP_OK
	} else {
		txt = fmt.Sprintf("%d - %v", code, E)
		color = ANSI_COLOR_RSP_ERR
	}

	TPC.smtpLog.log(txt, MODE_RECV, color)
	return
}

func (TPC loggedTextProtoConn) DotWriter() io.WriteCloser {
	return loggedWriteCloser{
		WriteCloser: TPC.Conn.DotWriter(),
		smtpLog:     TPC.smtpLog,
	}
}

/*
Can be used as a substitute CreateTextprotoConnFn to log the SMTP
conversation to a specified logger.

Example

	// create target logger
	pLog := log.New(os.Stdout, "", log.Ltime | log.Lmicroseconds)

	// establilsh connection to server
	HOST := "smtp.office365.com"
	iConn, _ = net.Dial("tcp4", HOST + ":587",)

	// create new session
	email.NewClient(
		iConn,
		email.LoginAuth("user name", "password"),
		HOST,
		email.TLSConfig(HOST),
		email.TextprotoLogged(pLog, true),   // true for ANSI colors, false for no colors
	)
*/
func TextprotoLogged(pLog *log.Logger, bColors bool) CreateTextprotoConnFn {

	return func(iConn net.Conn) TextProtoConn {
		return loggedTextProtoConn{
			Conn: textproto.NewConn(iConn),
			smtpLog: smtpLog{
				Logger: pLog,
				Colors: bColors,
			},
		}
	}
}
