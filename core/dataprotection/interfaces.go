package dataprotection

type DataProtector interface {
	Protect(data string) (protectedData string, err error)
	Unprotect(protectedData string) (data string, err error)
}
