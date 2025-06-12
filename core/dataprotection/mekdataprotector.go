package dataprotection

import (
	"errors"
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// MekDataProtector is a struct that provides methods to encrypt and decrypt data using the MEK (Master Encryption Key).
// It is intended to be used in the context of a web request, making it short-lived.
type MekDataProtector struct {
	mekStore          auth.MekStore
	encryptionService encryption.EncryptionService
	request           *http.Request
}

// CreateMekDataProtectorForRequest creates a new MekDataProtector instance for the given HTTP request.
// A MekDataProtector is intended to be short-lived and should be created for each request.
func CreateMekDataProtectorForRequest(mekStore auth.MekStore, encryptionService encryption.EncryptionService, r *http.Request) *MekDataProtector {
	return &MekDataProtector{
		mekStore:          mekStore,
		encryptionService: encryptionService,
		request:           r,
	}
}

// Protect encrypts the given piece of data using the MEK (Master Encryption Key) stored in the MekStore.
func (p *MekDataProtector) Protect(data string) (protectedData string, err error) {

	mek, err := p.mekStore.Retrieve(p.request)
	if err != nil || mek == "" {
		return "", errors.New("MEK not available")
	}

	// Encrypt the data using the MEK
	encryptedData, err := p.encryptionService.Encrypt(data, mek)
	if err != nil {
		return "", err
	}

	return encryptedData, nil
}

// Unprotect decrypts the given piece of data using the MEK (Master Encryption Key) stored in the MekStore.
func (p *MekDataProtector) Unprotect(protectedData string) (data string, err error) {

	mek, err := p.mekStore.Retrieve(p.request)
	if err != nil || mek == "" {
		return "", errors.New("MEK not available")
	}

	// Decrypt the data using the MEK
	decryptedData, err := p.encryptionService.Decrypt(protectedData, mek)
	if err != nil {
		return "", err
	}

	return decryptedData, nil
}
