package encryption

type Hasher interface {
	Hash(input string) (output string, salt string, err error)
	VerifyHash(input string, hash string, salt string) (isValid bool, err error)
}

type EncryptionService interface {
	Hasher
	Encrypt(plainText string, key string) (cipherText string, err error)
	Decrypt(cipherText string, key string) (plainText string, err error)
	EncryptBytes(plainData []byte, key string) (cipherData []byte, err error)
	DecryptBytes(cipherData []byte, key string) (plainData []byte, err error)
	GenerateKey() (key string, err error)
	GenerateKeyFromPassword(password string, salt string) (key string, err error)
	GenerateSalt() (saltBytes []byte, salt string, err error)
	GenerateRandomBytes(length int) (randomBytes []byte, err error)
	ConvertKeyToString(key []byte) (keyString string, err error)
	ConvertStringToKey(keyString string) (key []byte, err error)
}
