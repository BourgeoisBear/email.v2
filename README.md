# email.v2

[![GoDoc](https://godoc.org/github.com/BourgeoisBear/email.v2?status.svg)](https://godoc.org/github.com/BourgeoisBear/email.v2)

Fork of https://github.com/jordan-wright/email, a robust and flexible email library for Go.

### Reasons for Fork

- ripped out connection pooling (sending via long-term connections to SMTP servers has not been reliable)
- condensed multiple Send... methods into `SendWithConn()`.  `SendWithConn()` provides a generic way to utilize unauthenticated, SSL, and STARTTLS connections
- added `LoginAuth` authentication interface for use with Office 365

### Features

This package currently supports the following:

*  From, To, Bcc, and Cc fields
*  Email addresses in both "test@example.com" and "First Last &lt;test@example.com&gt;" format
*  Text and HTML Message Body
*  Attachments
*  Read Receipts
*  Custom headers

### Installation

```go get github.com/BourgeoisBear/email.v2```

### Examples

#### Sending email using typical STARTTLS server

```go

// 1. CONNECT TO THE SERVER
HOSTNAME := "mailserver.com"
AUTH     := smtp.PlainAuth("", Username, Password, HOSTNAME)
pTLSCfg  := &tls.Config{ ... }

// NOTE: Returns an *unencrypted* net.Conn
pConn, _ := net.Dial("tcp4", HOSTNAME + ":587")

// 2. CREATE THE MESSAGE
MSG := email.NewEmail()
MSG.From    = "Jordan Wright <test@gmail.com>"
MSG.To      = []string{"test@example.com"}
MSG.Bcc     = []string{"test_bcc@example.com"}
MSG.Cc      = []string{"test_cc@example.com"}
MSG.Subject = "Awesome Subject"
MSG.Text    = []byte("Text Body is, of course, supported!")
MSG.HTML    = []byte("<h1>Fancy HTML is supported, too!</h1>")

// 3. SEND MESSAGE
MSG.SendWithConn(pConn, AUTH, HOSTNAME, pTLSCfg)

```

#### Another Method for Creating an Email

You can also create an email directly by creating a struct as follows:

```go

MSG := &email.Email {
	To:      []string{"test@example.com"},
	From:    "Jordan Wright <test@gmail.com>",
	Subject: "Awesome Subject",
	Text:    []byte("Text Body is, of course, supported!"),
	HTML:    []byte("<h1>Fancy HTML is supported, too!</h1>"),
	Headers: textproto.MIMEHeader{},
}

```

#### Creating an Email From an io.Reader

You can also create an email from any type that implements the `io.Reader` interface by using `email.NewEmailFromReader`.

#### Attaching a File

```go
e := NewEmail()
e.AttachFile("test.txt")
```

### Documentation

[http://godoc.org/github.com/BourgeoisBear/email.v2](http://godoc.org/github.com/BourgeoisBear/email.v2)
