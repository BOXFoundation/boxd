// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package account

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	ctypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	bcrypto "github.com/BOXFoundation/boxd/crypto"
	"golang.org/x/crypto/scrypt"
)

const (
	scryptN     = 1 << 18
	scryptR     = 8
	scryptP     = 1
	scryptDklen = 32
)

// Account offers method to operate ecdsa keys stored in a keystore file path
type Account struct {
	Path     string
	Address  ctypes.Address
	PrivKey  *crypto.PrivateKey
	Unlocked bool
}

type keystorePassphrase struct {
	path         string
	pubicKeyHash string
	privateKey   *crypto.PrivateKey
}

type keyStoreJSON struct {
	ID      string     `json:"id"`
	Address string     `json:"address"`
	Crypto  cryptoJSON `json:"crypto"`
}

type cryptoJSON struct {
	Ciphertext   string           `json:"ciphertext"`
	Cipher       string           `json:"cipher"`
	Cipherparams cipherParamsJSON `json:"cipherparams"`
	Mac          string           `json:"mac"`
	KdfParams    kdfParamsJSON    `json:"kdfparams"`
}

type cipherParamsJSON struct {
	Iv string `json:"iv"`
}

type kdfParamsJSON struct {
	Salt  string `json:"salt"`
	Dklen int    `json:"dklen"`
	N     int    `json:"n"`
	R     int    `json:"r"`
	P     int    `json:"p"`
}

// Addr return addr
func (acc *Account) Addr() string {
	return acc.Address.String()
}

// AddrType returns Address interface of the account
func (acc *Account) AddrType() ctypes.Address {
	return acc.Address
}

// PubKeyHash returns Public Key Hash of the account
func (acc *Account) PubKeyHash() []byte {
	return acc.Address.Hash()
}

// PublicKey returns the account's public key in compressed byte format
func (acc *Account) PublicKey() []byte {
	return acc.PrivKey.PubKey().Serialize()
}

// PrivateKey returns the accounts private key in compressed byte format
func (acc *Account) PrivateKey() *crypto.PrivateKey {
	return acc.PrivKey
}

// SaveWithPassphrase save account with passphrase
func (acc *Account) SaveWithPassphrase(passphrase string) error {
	savePrivateKeyWithPassphrase(acc.PrivKey, passphrase, acc.Path)
	return nil
}

// UnlockWithPassphrase unlocks an account and generate its private key
func (acc *Account) UnlockWithPassphrase(passphrase string) error {
	privateKeyBytes, err := unlockPrivateKeyWithPassphrase(acc.Path, passphrase)
	if err != nil {
		return err
	}
	if acc.PrivKey == nil {
		acc.PrivKey = &crypto.PrivateKey{}
	}
	acc.PrivKey, _, err = crypto.KeyPairFromBytes(privateKeyBytes)
	if err != nil {
		return err
	}
	addr, err := ctypes.NewAddressFromPubKey(acc.PrivKey.PubKey())
	if err != nil {
		return err
	}
	if !bytes.Equal(addr.Hash(), acc.Address.Hash()) {
		return fmt.Errorf("Private key doesn't match address, the keystore file may be broken")
	}
	acc.Unlocked = true
	return nil
}

var _ crypto.Signer = (*Account)(nil)

// Sign calculates an ECDSA signature of messageHash using privateKey.
// returns error if account is locked or sign process failed
func (acc *Account) Sign(messageHash *crypto.HashType) (*crypto.Signature, error) {
	if acc.Unlocked == false || acc.PrivKey == nil {
		return nil, fmt.Errorf("Address unlocked")
	}
	return crypto.Sign(acc.PrivKey, messageHash)
}

// NewAccountFromFile create account from file.
func NewAccountFromFile(filePath string) (*Account, error) {
	pubKeyHashString, err := GetKeystoreAddress(filePath)
	if err != nil {
		return nil, err
	}
	pubKeyHashBytes, err := hex.DecodeString(pubKeyHashString)
	if err != nil {
		return nil, err
	}
	addr, err := ctypes.NewAddressPubKeyHash(pubKeyHashBytes)
	if err != nil {
		return nil, err
	}
	acc := &Account{
		Path:     filePath,
		Address:  addr,
		Unlocked: false,
	}
	return acc, nil
}

func savePrivateKeyWithPassphrase(privatekey *bcrypto.PrivateKey, passphrase, path string) error {
	addr, err := ctypes.NewAddressFromPubKey(privatekey.PubKey())
	if err != nil {
		return err
	}
	cpt, err := newCryptoJSON(privatekey, passphrase)
	if err != nil {
		return err
	}
	ksJSON := &keyStoreJSON{
		Crypto:  cpt,
		Address: hex.EncodeToString(addr.Hash()),
	}
	content, err := json.Marshal(ksJSON)
	if err != nil {
		return err
	}
	tmpPath, err := tryWriteTempFile(path, content)
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func tryWriteTempFile(path string, content []byte) (string, error) {
	const dirPerm = 0700
	dir := filepath.Dir(path)
	filename := filepath.Base(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", err
	}
	f, err := ioutil.TempFile(dir, fmt.Sprintf(".%s.tmp", filename))
	if err != nil {
		return "", err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func newCryptoJSON(privateKey *bcrypto.PrivateKey, passphrase string) (cryptoJSON, error) {
	if len(passphrase) == 0 {
		return cryptoJSON{}, fmt.Errorf("Passphrase should not be empty")
	}
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return cryptoJSON{}, err
	}
	derivedKey, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, scryptDklen)
	if err != nil {
		return cryptoJSON{}, err
	}
	aesKey := derivedKey[:16]

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return cryptoJSON{}, err
	}
	cipherText, err := aesCtr(aesKey, privateKey.Serialize(), iv)
	if err != nil {
		return cryptoJSON{}, err
	}
	mac := bcrypto.Sha256Multi(derivedKey[16:32], cipherText)
	kdfParam := kdfParamsJSON{
		Salt:  hex.EncodeToString(salt),
		Dklen: scryptDklen,
		N:     scryptN,
		R:     scryptR,
		P:     scryptP,
	}
	cipherParam := cipherParamsJSON{
		Iv: hex.EncodeToString(iv),
	}
	cpt := cryptoJSON{
		Ciphertext:   hex.EncodeToString(cipherText),
		Cipherparams: cipherParam,
		Cipher:       "aes-128-ctr",
		KdfParams:    kdfParam,
		Mac:          hex.EncodeToString(mac),
	}
	return cpt, nil
}

func unlockPrivateKeyWithPassphrase(path, passphrase string) ([]byte, error) {
	ksJSON, err := readKeystoreJSON(path)
	if err != nil {
		return nil, err
	}
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("Passphrase should not be empty")
	}
	cpt := ksJSON.Crypto
	kdfParams := cpt.KdfParams
	salt, err := hex.DecodeString(kdfParams.Salt)
	if err != nil {
		return nil, err
	}
	derivedKey, err := scrypt.Key(
		[]byte(passphrase),
		salt,
		kdfParams.N,
		kdfParams.R,
		kdfParams.P,
		kdfParams.Dklen,
	)
	if err != nil {
		return nil, err
	}
	cipherText, err := hex.DecodeString(cpt.Ciphertext)
	if err != nil {
		return nil, err
	}
	mac := bcrypto.Sha256Multi(derivedKey[16:32], cipherText)
	if hex.EncodeToString(mac) != cpt.Mac {
		return nil, fmt.Errorf("Incorrect Passphrase")
	}
	aesKey := derivedKey[:16]
	iv, err := hex.DecodeString(cpt.Cipherparams.Iv)
	if err != nil {
		return nil, err
	}

	originText, err := aesCtr(aesKey, cipherText, iv)
	if err != nil {
		return nil, err
	}
	return originText, nil
}

func aesCtr(key, text, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	output := make([]byte, len(text))
	stream.XORKeyStream(output, text)
	return output, err
}

// GetKeystoreAddress gets the address info from a keystore json file
func GetKeystoreAddress(path string) (string, error) {
	ksJSON, err := readKeystoreJSON(path)
	if err != nil {
		return "", err
	}
	return ksJSON.Address, nil
}

func readKeystoreJSON(path string) (*keyStoreJSON, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ksJSON keyStoreJSON
	if err = json.Unmarshal(fileContent, &ksJSON); err != nil {
		return nil, err
	}
	return &ksJSON, nil
}

func (ks *keystorePassphrase) Marshal() ([]byte, error) {
	return json.Marshal(ks)
}
