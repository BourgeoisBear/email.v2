package email

import (
	"os"
	"strings"
	"testing"

	"encoding/json"
	"fmt"

	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"

	"crypto/rand"
	"net/mail"
	"net/textproto"
)

type testClientCfg struct {
	SMTPClientConfig
	From string
}

type testSettings struct {
	To       []string
	ToCc     []string
	ToBcc    []string
	Subject  string
	Body     string
	Accounts map[string]testClientCfg
}

func loadSettings() (testSettings, error) {
	var ret testSettings
	bsSettings, err := os.ReadFile("./email_test_settings.json")
	if err != nil {
		return ret, err
	}
	err = json.Unmarshal(bsSettings, &ret)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func dummyEmail() *Email {
	pMsg := NewEmail()
	pMsg.From = "test@test.com"
	pMsg.To = []string{"recipient@test.com", "recipient2@test.com"}
	pMsg.Cc = []string{"cc1@test.com", "cc2@test.com"}
	pMsg.Bcc = []string{"bcc1@test.com", "bcc2@test.com"}
	pMsg.Subject = "Test Subject"
	return pMsg
}

func basicTests(t *testing.T, msgIn *Email) *mail.Message {

	raw, err := msgIn.Bytes()
	if err != nil {
		t.Fatal("Failed to render message: ", msgIn)
	}

	msgOut, err := mail.ReadMessage(bytes.NewBuffer(raw))
	if err != nil {
		t.Fatal("Could not parse rendered message: ", err)
	}

	expectedHeaders := map[string]string{
		"From":    msgIn.From,
		"To":      strings.Join(msgIn.To, ", "),
		"Cc":      strings.Join(msgIn.Cc, ", "),
		"Subject": msgIn.Subject,
	}

	for header, expected := range expectedHeaders {
		if val := msgOut.Header.Get(header); val != expected {
			t.Errorf("Wrong value for message header %s: %v != %v", header, expected, val)
		}
	}
	return msgOut
}

func TestEmailText(t *testing.T) {

	msgIn := dummyEmail()
	msgIn.Text = []byte("Text Body is, of course, supported!\n")
	msgOut := basicTests(t, msgIn)

	// Were the right headers set?
	ct := msgOut.Header.Get("Content-type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "text/plain" {
		t.Fatalf("Content-type expected \"text/plain\", not %v", mt)
	}
}

func TestEmailHTML(t *testing.T) {

	msgIn := dummyEmail()
	msgIn.HTML = []byte("<h1>Fancy Html is supported, too!</h1>\n")
	msgOut := basicTests(t, msgIn)

	// Were the right headers set?
	ct := msgOut.Header.Get("Content-type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("Content-type header is invalid: %#v", ct)
	} else if mt != "text/html" {
		t.Fatalf("Content-type expected \"text/html\", not %v", mt)
	}
}

func TestEmailTextAttachment(t *testing.T) {

	msgIn := dummyEmail()
	msgIn.Text = []byte("Text Body is, of course, supported!\n")

	_, err := msgIn.Attach(
		bytes.NewBufferString("Rad attachment"),
		"rad.txt",
		"text/plain; charset=utf-8",
	)
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}

	msgOut := basicTests(t, msgIn)

	// Were the right headers set?
	ct := msgOut.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatalf("Invalid or missing boundary parameter: %#v", b)
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msgOut.Body, params["boundary"])

	text, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find text component of email: %s", err)
	}

	// Does the text portion match what we expect?
	mt, _, err = mime.ParseMediaType(text.Header.Get("Content-type"))
	if err != nil {
		t.Fatal("Could not parse message's Content-Type")
	} else if mt != "text/plain" {
		t.Fatal("Message missing text/plain")
	}
	plainText, err := io.ReadAll(text)
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	if !bytes.Equal(plainText, []byte("Text Body is, of course, supported!\r\n")) {
		t.Fatalf("Plain text is broken: %#q", plainText)
	}

	// Check attachments.
	_, err = mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only text and one attachment!")
	}
}

func TestEmailTextHtmlAttachment(t *testing.T) {

	msgIn := dummyEmail()
	msgIn.Text = []byte("Text Body is, of course, supported!\n")
	msgIn.HTML = []byte("<h1>Fancy Html is supported, too!</h1>\n")

	_, err := msgIn.Attach(
		bytes.NewBufferString("Rad attachment"),
		"rad.txt",
		"text/plain; charset=utf-8",
	)
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}

	msgOut := basicTests(t, msgIn)

	// Were the right headers set?
	ct := msgOut.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatal("Unexpected empty boundary parameter")
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msgOut.Body, params["boundary"])

	text, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find text component of email: %s", err)
	}

	// Does the text portion match what we expect?
	mt, params, err = mime.ParseMediaType(text.Header.Get("Content-type"))
	if err != nil {
		t.Fatal("Could not parse message's Content-Type")
	} else if mt != "multipart/alternative" {
		t.Fatal("Message missing multipart/alternative")
	}
	mpReader := multipart.NewReader(text, params["boundary"])
	part, err := mpReader.NextPart()
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	plainText, err := io.ReadAll(part)
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	if !bytes.Equal(plainText, []byte("Text Body is, of course, supported!\r\n")) {
		t.Fatalf("Plain text is broken: %#q", plainText)
	}

	// Check attachments.
	_, err = mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only text and one attachment!")
	}
}

func TestEmailAttachment(t *testing.T) {

	msgIn := dummyEmail()

	_, err := msgIn.Attach(
		bytes.NewBufferString("Rad attachment"),
		"rad.txt",
		"text/plain; charset=utf-8",
	)
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}
	msgOut := basicTests(t, msgIn)

	// Were the right headers set?
	ct := msgOut.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatal("Unexpected empty boundary parameter")
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msgOut.Body, params["boundary"])

	// Check attachments.
	_, err = mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only one attachment!")
	}
}

func TestEmailFromReader(t *testing.T) {

	msgExpect := &Email{
		From:    "Jordan Wright <jmwright798@gmail.com>",
		To:      []string{"Jordan Wright <jmwright798@gmail.com>"},
		Subject: "Test Subject",
		Text:    []byte("This is a test email with HTML Formatting. It also has very long lines so\nthat the content must be wrapped if using quoted-printable decoding.\n"),
		HTML:    []byte("<div dir=\"ltr\">This is a test email with <b>HTML Formatting.</b>\u00a0It also has very long lines so that the content must be wrapped if using quoted-printable decoding.</div>\n"),
	}
	raw := []byte(`MIME-Version: 1.0
Subject: Test Subject
From: Jordan Wright <jmwright798@gmail.com>
To: Jordan Wright <jmwright798@gmail.com>
Content-Type: multipart/alternative; boundary=001a114fb3fc42fd6b051f834280

--001a114fb3fc42fd6b051f834280
Content-Type: text/plain; charset=UTF-8

This is a test email with HTML Formatting. It also has very long lines so
that the content must be wrapped if using quoted-printable decoding.

--001a114fb3fc42fd6b051f834280
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

<div dir=3D"ltr">This is a test email with <b>HTML Formatting.</b>=C2=A0It =
also has very long lines so that the content must be wrapped if using quote=
d-printable decoding.</div>

--001a114fb3fc42fd6b051f834280--`)
	msgRead, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if msgRead.Subject != msgExpect.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", msgRead.Subject, msgExpect.Subject)
	}
	if !bytes.Equal(msgRead.Text, msgExpect.Text) {
		t.Fatalf("Incorrect text: %#q != %#q", msgRead.Text, msgExpect.Text)
	}
	if !bytes.Equal(msgRead.HTML, msgExpect.HTML) {
		t.Fatalf("Incorrect HTML: %#q != %#q", msgRead.HTML, msgExpect.HTML)
	}
	if msgRead.From != msgExpect.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", msgRead.From, msgExpect.From)
	}
}

func TestNonAsciiEmailFromReader(t *testing.T) {

	msgExpect := &Email{
		From:    "Mrs ValÃ©rie Dupont <valerie.dupont@example.com>",
		To:      []string{"Anaïs <anais@example.org>"},
		Cc:      []string{"Patrik Fältström <paf@example.com>"},
		Subject: "Test Subject",
		Text:    []byte("This is a test message!"),
	}
	raw := []byte(`
	MIME-Version: 1.0
Subject: =?UTF-8?Q?Test Subject?=
From: Mrs =?ISO-8859-1?Q?Val=C3=A9rie=20Dupont?= <valerie.dupont@example.com>
To: =?utf-8?q?Ana=C3=AFs?= <anais@example.org>
Cc: =?ISO-8859-1?Q?Patrik_F=E4ltstr=F6m?= <paf@example.com>
Content-type: text/plain; charset=ISO-8859-1

This is a test message!`)
	msgRead, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if msgRead.Subject != msgExpect.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", msgRead.Subject, msgExpect.Subject)
	}
	if msgRead.From != msgExpect.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", msgRead.From, msgExpect.From)
	}
	if msgRead.To[0] != msgExpect.To[0] {
		t.Fatalf("Incorrect \"To\": %#q != %#q", msgRead.To, msgExpect.To)
	}
	if msgRead.Cc[0] != msgExpect.Cc[0] {
		t.Fatalf("Incorrect \"Cc\": %#q != %#q", msgRead.Cc, msgExpect.Cc)
	}
}

func TestNonMultipartEmailFromReader(t *testing.T) {

	msgExpect := &Email{
		Subject: "Example Subject (no MIME Type)",
		Headers: textproto.MIMEHeader{},
		Text:    []byte("This is a test message!"),
	}
	msgExpect.Headers.Add("Content-Type", "text/plain; charset=us-ascii")
	msgExpect.Headers.Add("Message-ID", "<foobar@example.com>")
	raw := []byte(`From: "Foo Bar" <foobar@example.com>
Content-Type: text/plain
To: foobar@example.com
Subject: Example Subject (no MIME Type)
Message-ID: <foobar@example.com>

This is a test message!`)
	msgRead, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if msgExpect.Subject != msgRead.Subject {
		t.Errorf("Incorrect subject. %#q != %#q\n", msgExpect.Subject, msgRead.Subject)
	}
	if !bytes.Equal(msgExpect.Text, msgRead.Text) {
		t.Errorf("Incorrect body. %#q != %#q\n", msgExpect.Text, msgRead.Text)
	}
	if msgExpect.Headers.Get("Message-ID") != msgRead.Headers.Get("Message-ID") {
		t.Errorf("Incorrect message ID. %#q != %#q\n", msgExpect.Headers.Get("Message-ID"), msgRead.Headers.Get("Message-ID"))
	}
}

func TestBase64EmailFromReader(t *testing.T) {

	msgExpect := &Email{
		From:    "Jordan Wright <jmwright798@gmail.com>",
		To:      []string{"Jordan Wright <jmwright798@gmail.com>"},
		Subject: "Test Subject",
		Text:    []byte("This is a test email with HTML Formatting. It also has very long lines so that the content must be wrapped if using quoted-printable decoding."),
		HTML:    []byte("<div dir=\"ltr\">This is a test email with <b>HTML Formatting.</b>\u00a0It also has very long lines so that the content must be wrapped if using quoted-printable decoding.</div>\n"),
	}
	raw := []byte(`MIME-Version: 1.0
Subject: Test Subject
From: Jordan Wright <jmwright798@gmail.com>
To: Jordan Wright <jmwright798@gmail.com>
Content-Type: multipart/alternative; boundary=001a114fb3fc42fd6b051f834280

--001a114fb3fc42fd6b051f834280
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: base64

VGhpcyBpcyBhIHRlc3QgZW1haWwgd2l0aCBIVE1MIEZvcm1hdHRpbmcuIEl0IGFsc28gaGFzIHZl
cnkgbG9uZyBsaW5lcyBzbyB0aGF0IHRoZSBjb250ZW50IG11c3QgYmUgd3JhcHBlZCBpZiB1c2lu
ZyBxdW90ZWQtcHJpbnRhYmxlIGRlY29kaW5nLg==

--001a114fb3fc42fd6b051f834280
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

<div dir=3D"ltr">This is a test email with <b>HTML Formatting.</b>=C2=A0It =
also has very long lines so that the content must be wrapped if using quote=
d-printable decoding.</div>

--001a114fb3fc42fd6b051f834280--`)
	msgRead, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if msgRead.Subject != msgExpect.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", msgRead.Subject, msgExpect.Subject)
	}
	if !bytes.Equal(msgRead.Text, msgExpect.Text) {
		t.Fatalf("Incorrect text: %#q != %#q", msgRead.Text, msgExpect.Text)
	}
	if !bytes.Equal(msgRead.HTML, msgExpect.HTML) {
		t.Fatalf("Incorrect HTML: %#q != %#q", msgRead.HTML, msgExpect.HTML)
	}
	if msgRead.From != msgExpect.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", msgRead.From, msgExpect.From)
	}
}

func TestSend(t *testing.T) {

	var err error
	defer func() {
		if err != nil {
			t.Error(err)
		}
	}()

	cfg, err := loadSettings()
	if err != nil {
		return
	}

	// TEST MESSAGE
	msg := Email{
		Headers: textproto.MIMEHeader{},
		To:      cfg.To,
		Text:    []byte(cfg.Body),
	}

	// TRY SENDING ON EACH GIVEN ACCOUNT
	for szAcctLbl, oAcct := range cfg.Accounts {

		t.Log("TestSend: " + szAcctLbl)

		msg.From = oAcct.From
		msg.Subject = fmt.Sprintf("%s - %d", cfg.Subject, 1)

		msg2 := msg
		msg2.Subject = fmt.Sprintf("%s - %d", cfg.Subject, 2)

		// TEST SENDING MULTIPLE MESSAGES INSIDE SINGLE SMTP SESSION
		oAcct.SMTPLog = "-"
		oAcct.TimeoutMsec = 7000
		err = oAcct.SimpleSend(&msg, &msg2)
		if err != nil {
			return
		}

		t.Log("TestSend: " + szAcctLbl + " - SUCCESS!")
	}
}

func Test_base64Wrap(t *testing.T) {
	file := "I'm a file long enough to force the function to wrap a\n" +
		"couple of lines, but I stop short of the end of one line and\n" +
		"have some padding dangling at the end."
	encoded := "SSdtIGEgZmlsZSBsb25nIGVub3VnaCB0byBmb3JjZSB0aGUgZnVuY3Rpb24gdG8gd3JhcCBhCmNv\r\n" +
		"dXBsZSBvZiBsaW5lcywgYnV0IEkgc3RvcCBzaG9ydCBvZiB0aGUgZW5kIG9mIG9uZSBsaW5lIGFu\r\n" +
		"ZApoYXZlIHNvbWUgcGFkZGluZyBkYW5nbGluZyBhdCB0aGUgZW5kLg==\r\n"

	var buf bytes.Buffer
	base64Wrap(&buf, []byte(file))
	if !bytes.Equal(buf.Bytes(), []byte(encoded)) {
		t.Fatalf("Encoded file does not match expected: %#q != %#q", string(buf.Bytes()), encoded)
	}
}

// *Since the mime library in use by ```email``` is now in the stdlib, this test is deprecated
func Test_quotedPrintEncode(t *testing.T) {
	var buf bytes.Buffer
	text := []byte("Dear reader!\n\n" +
		"This is a test email to try and capture some of the corner cases that exist within\n" +
		"the quoted-printable encoding.\n" +
		"There are some wacky parts like =, and this input assumes UNIX line breaks so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's a fish: 🐟\n")
	expected := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist=\r\n" +
		" within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =3D, and this input assumes UNIX line break=\r\n" +
		"s so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's=\r\n" +
		" a fish: =F0=9F=90=9F\r\n")
	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write(text); err != nil {
		t.Fatal("quotePrintEncode: ", err)
	}
	if err := qp.Close(); err != nil {
		t.Fatal("Error closing writer", err)
	}
	if b := buf.Bytes(); !bytes.Equal(b, expected) {
		t.Errorf("quotedPrintEncode generated incorrect results: %#q != %#q", b, expected)
	}
}

func TestMultipartNoContentType(t *testing.T) {
	raw := []byte(`From: Mikhail Gusarov <dottedmag@dottedmag.net>
To: notmuch@notmuchmail.org
References: <20091117190054.GU3165@dottiness.seas.harvard.edu>
Date: Wed, 18 Nov 2009 01:02:38 +0600
Message-ID: <87iqd9rn3l.fsf@vertex.dottedmag>
MIME-Version: 1.0
Subject: Re: [notmuch] Working with Maildir storage?
Content-Type: multipart/mixed; boundary="===============1958295626=="

--===============1958295626==
Content-Type: multipart/signed; boundary="=-=-=";
    micalg=pgp-sha1; protocol="application/pgp-signature"

--=-=-=
Content-Transfer-Encoding: quoted-printable

Twas brillig at 14:00:54 17.11.2009 UTC-05 when lars@seas.harvard.edu did g=
yre and gimble:

--=-=-=
Content-Type: application/pgp-signature

-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.9 (GNU/Linux)

iQIcBAEBAgAGBQJLAvNOAAoJEJ0g9lA+M4iIjLYQAKp0PXEgl3JMOEBisH52AsIK
=/ksP
-----END PGP SIGNATURE-----
--=-=-=--

--===============1958295626==
Content-Type: text/plain; charset="us-ascii"
MIME-Version: 1.0
Content-Transfer-Encoding: 7bit
Content-Disposition: inline

Testing!
--===============1958295626==--
`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error when parsing email %s", err.Error())
	}
	if !bytes.Equal(e.Text, []byte("Testing!")) {
		t.Fatalf("Error incorrect text: %#q != %#q\n", e.Text, "Testing!")
	}
}

// *Since the mime library in use by ```email``` is now in the stdlib, this test is deprecated
func Test_quotedPrintDecode(t *testing.T) {
	text := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist=\r\n" +
		" within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =3D, and this input assumes UNIX line break=\r\n" +
		"s so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's=\r\n" +
		" a fish: =F0=9F=90=9F\r\n")
	expected := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =, and this input assumes UNIX line breaks so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's a fish: 🐟\r\n")
	qp := quotedprintable.NewReader(bytes.NewReader(text))
	got, err := io.ReadAll(qp)
	if err != nil {
		t.Fatal("quotePrintDecode: ", err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("quotedPrintDecode generated incorrect results: %#q != %#q", got, expected)
	}
}

func Benchmark_base64Wrap(b *testing.B) {
	// Reasonable base case; 128K random bytes
	file := make([]byte, 128*1024)
	if _, err := rand.Read(file); err != nil {
		panic(err)
	}
	for i := 0; i <= b.N; i++ {
		base64Wrap(io.Discard, file)
	}
}

func TestParseSender(t *testing.T) {
	var cases = []struct {
		e      Email
		want   string
		haserr bool
	}{
		{
			Email{From: "from@test.com"},
			"from@test.com",
			false,
		},
		{
			Email{Sender: "sender@test.com", From: "from@test.com"},
			"sender@test.com",
			false,
		},
		{
			Email{Sender: "bad_address_sender"},
			"",
			true,
		},
		{
			Email{Sender: "good@sender.com", From: "bad_address_from"},
			"good@sender.com",
			false,
		},
	}

	for i, testcase := range cases {

		got, err := testcase.e.ParseSender()

		if (got.Address != testcase.want) || ((err != nil) != testcase.haserr) {
			t.Errorf(
				`%d: got(%s) != want(%s) or error "%t" != "%t"`,
				i+1,
				got.Address,
				testcase.want,
				err != nil,
				testcase.haserr,
			)
		}
	}
}
