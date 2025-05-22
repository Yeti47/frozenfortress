package encryption

type Hasher interface {
	Hash(input string) (output string, salt string, err error)
}

type EncryptionService interface {
	Hasher
	Encrypt(plainText string, key string) (cipherText string, err error)
	Decrypt(cipherText string, key string) (plainText string, err error)
	GenerateKey() (key string, err error)
	GenerateKeyFromPassword(password string) (key string, err error)
	GenerateSalt() (saltBytes []byte, salt string, err error)
}
