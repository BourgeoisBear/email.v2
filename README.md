# email.v2

[![GoDoc](https://godoc.org/github.com/BourgeoisBear/email.v2?status.svg)](https://godoc.org/github.com/BourgeoisBear/email.v2)

Fork of https://github.com/jordan-wright/email, a robust and flexible email library for Go.

### Reasons for Fork

* ripped out connection pooling (sending via long-term connections to SMTP servers has not been reliable)
* condensed multiple Send... methods into NewClient() & Send() to
	* provide a more generic way of establishing unauthenticated, SSL, and STARTTLS connections
	* send multiple messages from within a single established SMTP session
	* make direct use of outside net.Conn interfaces, so as to set dial and I/O deadlines
* added `LoginAuth` authentication interface for use with Office 365
* added `TextprotoLogged` for full logging of SMTP traffic

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

### Usage

See the godoc.org documentation for examples:

[http://godoc.org/github.com/BourgeoisBear/email.v2](http://godoc.org/github.com/BourgeoisBear/email.v2)

### Testing

To run unit tests, add the proper credentials to `email_test_settings.json` for accounts you choose to test with.  Examples for O365, GMAIL, & CUSTOM have been provided.
