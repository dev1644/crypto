package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keylen  = 32
	saltlen = 8
)

// EncryptManager handles file encryption and decryption
type EncryptManager struct {
	passphrase []byte
}

// NewEncryptManager creates a new EncryptManager
func NewEncryptManager(passphrase string) *EncryptManager {
	return &EncryptManager{[]byte(passphrase)}
}

// EncryptGCM encrypts given io.Reader using AES256-GCM
// the resultant encrypted bytes, nonce, and cipher are returned
func (e *EncryptManager) EncryptGCM(r io.Reader) ([]byte, []byte, []byte, error) {
	// create a 32bit cipher key allowing usage for AES256-GCM
	cipherKeyBytes := make([]byte, 32)
	if _, err := rand.Read(cipherKeyBytes); err != nil {
		return nil, nil, nil, err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, nil, err
	}
	block, err := aes.NewCipher(cipherKeyBytes)
	if err != nil {
		return nil, nil, nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	dataToEncrypt, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, nil, err
	}
	return aesGCM.Seal(nil, nonce, dataToEncrypt, nil), nonce, cipherKeyBytes, nil
}

// DecryptGCM is used to decrypt the given io.Reader using a specified key and nonce
// the key and nonce are expected to be in the format of hex.EncodeToString
func (e *EncryptManager) DecryptGCM(r io.Reader, key, nonce string) ([]byte, error) {
	// decode the key
	decodedKey, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	// decode the nonce
	decodedNonce, err := hex.DecodeString(nonce)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	encryptedData, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return aesGCM.Open(nil, decodedNonce, encryptedData, nil)
}

// EncryptCFB encrypts given io.Reader using AES256CFB
// the resultant bytes are returned
func (e *EncryptManager) EncryptCFB(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("invalid content provided")
	}

	// generate salt, encrypt password for use as a key for a cipher
	salt := make([]byte, saltlen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := pbkdf2.Key(e.passphrase, salt, 4096, keylen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// read original content
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// generate an intialization vector for encryption
	encrypted := make([]byte, aes.BlockSize+len(b))
	iv := encrypted[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// encrypt
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(encrypted[aes.BlockSize:], b)

	// attach salt to end of encrypted content
	encrypted = append(encrypted, salt...)

	return encrypted, nil
}

// DecryptCFB decrypts given io.Reader which was encrypted using AES256-CFB
// the resulting decrypt bytes are returned
func (e *EncryptManager) DecryptCFB(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("invalid content provided")
	}

	// read raw contents
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// retrieve and remove salt
	salt := raw[len(raw)-saltlen:]
	raw = raw[:len(raw)-saltlen]

	// generate cipher
	key := pbkdf2.Key(e.passphrase, salt, 4096, keylen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// decrypt contents
	stream := cipher.NewCFBDecrypter(block, raw[:aes.BlockSize])
	decrypted := make([]byte, len(raw)-aes.BlockSize)
	stream.XORKeyStream(decrypted, raw[aes.BlockSize:])

	return decrypted, nil
}
