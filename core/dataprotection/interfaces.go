package dataprotection

type DataProtector interface {
	Protect(data string) (protectedData string, err error)
	Unprotect(protectedData string) (data string, err error)
	ProtectBytes(data []byte) (protectedData []byte, err error)
	UnprotectBytes(protectedData []byte) (data []byte, err error)
}
