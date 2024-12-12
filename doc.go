/*
Yet another SMTP client!

Simple Usage

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

Advanced Usage

	See implementation of SMTPClientConfig.SimpleSend()
*/
package email
