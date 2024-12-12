package email

type MailErr int

const (
	ErrMissingToOrFrom MailErr = iota
	ErrMissingBoundary
	ErrMissingContentType
	ErrSTARTTLSNotOffered
	ErrInvalidSMTPMode
	ErrUnencryptedConn
	ErrWrongHostname
	ErrUnexpectedServerChallenge
	ErrLateHELO
	ErrHasCRLF
)

func (e MailErr) Error() string {
	switch e {
	case ErrMissingToOrFrom:
		return "must specify at least one `From` address and one `To` address"
	case ErrMissingBoundary:
		return "no boundary found for multipart entity"
	case ErrMissingContentType:
		return "no Content-Type found for MIME entity"
	case ErrSTARTTLSNotOffered:
		return "STARTTLS not offered by server"
	case ErrInvalidSMTPMode:
		return "valid SMTPClientModes are: UNENCRYPTED, STARTTLS, or FORCETLS"
	case ErrUnencryptedConn:
		return "unencrypted connection"
	case ErrWrongHostname:
		return "wrong host name"
	case ErrUnexpectedServerChallenge:
		return "unexpected server challenge"
	case ErrLateHELO:
		return "HELO called after other methods"
	case ErrHasCRLF:
		return "line must not contain CR or LF"
	}
	return "unknown MailErr"
}
