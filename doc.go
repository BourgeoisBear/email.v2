
/*
An E-Mail Interface for Nerds

How to Use

	1. Open Connection (encrypted or not)
	2. Establish SMTP Session (EHLO, STARTTLS, etc)
	3. Send Messages
	4. Close SMTP Session & Network Connection

Example

	// BOILERPLATE

		var E error

		smtpServer := "mailserver.com"
		iAuth      := email.LoginAuth(Username, Password)
		pTLSCfg    := email.TLSConfig(smtpServer)

	// [1 & 2]: ESTABLISH SESSION

		switch( mode ) {

		case "starttls":

			// [1]: open an unencrypted network connection
			iConn, E := net.Dial("tcp4", smtpServer + ":587")

			// [2]: negotiate TLS in SMTP session
			SESS, E := email.NewClient(iConn, iAuth, smtpServer, pTLSCfg, nil)

		case "forcetls":

			// [1]: open a TLS-secured network connection
			iConn, E := tls.Dial("tcp4", smtpServer + ":465", pTLSCfg)

			// [2]: establish SMTP session
			SESS, E := email.NewClient(iConn, iAuth, smtpServer, nil, nil)

		case "UNENCRYPTED":

			// [1]: open an unencrypted network connection
			iConn, E := net.Dial("tcp4", smtpServer + ":25")

			// [2]: establish SMTP session
			SESS, E := email.NewClient(iConn, iAuth, smtpServer, nil, nil)
		}

	// [3]: SEND MESSAGE(S)

		MSG := email.Email{ ... }
		E = SESS.Send(MSG)

	// [4]: CLOSE SMTP SESSION WHEN FINISHED SENDING
	// NOTE: this also closes the underlying network connection

		E = SESS.Quit()
*/
package email
