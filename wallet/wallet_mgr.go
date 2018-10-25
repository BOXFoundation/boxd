// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	btypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"golang.org/x/crypto/ssh/terminal"
)

// Manager is a directory based type to manipulate account
// Operation add/delete/query, import/export and sign are supported
type Manager struct {
	path     string
	accounts map[string]*Account
}

// NewWalletManager creates a wallet manager from files in the path
func NewWalletManager(path string) (*Manager, error) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		if os.IsNotExist(err) {
			errCreate := os.Mkdir(path, os.ModePerm)
			if errCreate != nil {
				return nil, errCreate
			}
		} else {
			return nil, err
		}
	}
	wlt := Manager{path: path}
	return &wlt, wlt.loadAccounts()
}

func (wlt *Manager) loadAccounts() error {
	files := getKeystoreFilePaths(wlt.path)
	accounts := make([]*Account, 0)
	for _, filePath := range files {
		account, err := NewAccountFromFile(filePath)
		if err == nil {
			accounts = append(accounts, account)
		}
	}
	wlt.accounts = make(map[string]*Account)
	for _, account := range accounts {
		wlt.accounts[account.addr] = account
	}
	return nil
}

func getKeystoreFilePaths(baseDir string) (files []string) {
	dir, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return
	}
	files = make([]string, 0)
	sep := string(os.PathSeparator)
	for _, fi := range dir {
		if !fi.IsDir() {
			name := fi.Name()
			if strings.HasSuffix(name, ".keystore") {
				files = append(files, baseDir+sep+name)
			}
		}
	}
	return
}

// ListAccounts returns all the addresses of keystore files in directory
func (wlt *Manager) ListAccounts() []string {
	addrs := make([]string, len(wlt.accounts))
	i := 0
	for addr := range wlt.accounts {
		addrs[i] = addr
		i++
	}
	return addrs
}

// NewAccount creates a ecdsa key pair and store them in a file encrypted
// by the passphrase user entered
// returns a hexstring format public key hash, address and error
func (wlt *Manager) NewAccount(passphrase string) (string, string, error) {
	privateKey, publicKey, err := crypto.NewKeyPair()
	if err != nil {
		return "", "", err
	}
	address, err := btypes.NewAddressFromPubKey(publicKey)
	if err != nil {
		return "", "", err
	}
	account := &Account{
		path:     path.Join(wlt.path, fmt.Sprintf("%x.keystore", address.ScriptAddress())),
		privKey:  privateKey,
		addr:     hex.EncodeToString(address.ScriptAddress()),
		unlocked: true,
	}
	if err := account.saveWithPassphrase(passphrase); err != nil {
		return "", "", err
	}
	return account.addr, address.String(), nil
}

// DumpPrivKey returns an account's private key bytes in hex string format
func (wlt *Manager) DumpPrivKey(address, passphrase string) (string, error) {
	acc, ok := wlt.accounts[address]
	if !ok {
		return "", fmt.Errorf("Address not found: %s", address)
	}
	if err := acc.UnlockWithPassphrase(passphrase); err != nil {
		return "", err
	}
	return hex.EncodeToString(acc.privKey.Serialize()), nil
}

// GetAccount checks if this Manager contains this public key
// and returns the related account if it exists
func (wlt *Manager) GetAccount(pubKeyHash string) (account *Account, exist bool) {
	account, exist = wlt.accounts[pubKeyHash]
	return
}

// Sign create signature of message bytes using private key related to input public key
func (wlt *Manager) Sign(msg []byte, pubKeyHash, passphrase string) ([]byte, error) {
	account, exist := wlt.GetAccount(pubKeyHash)
	if !exist {
		return nil, fmt.Errorf("Not managed account: %s", pubKeyHash)
	}
	if len(msg) != crypto.HashSize {
		return nil, fmt.Errorf("Invalid message digest length, must be %d bytes", crypto.HashSize)
	}
	hash := &crypto.HashType{}
	hash.SetBytes(msg)

	account.UnlockWithPassphrase(passphrase)

	sig, err := crypto.Sign(account.privKey, hash)
	if err != nil {
		return nil, err
	}
	return sig.Serialize(), nil
}

// Account offers method to operate ecdsa keys stored in a keystore file path
type Account struct {
	path     string
	addr     string
	privKey  *crypto.PrivateKey
	unlocked bool
}

// NewAccountFromFile create account from file.
func NewAccountFromFile(filePath string) (*Account, error) {
	addr, err := GetKeystoreAddress(filePath)
	if err != nil {
		return nil, err
	}
	acc := &Account{
		path:     filePath,
		addr:     addr,
		unlocked: false,
	}
	return acc, nil
}

// Addr return addr
func (acc *Account) Addr() string {
	return acc.addr
}

// PublicKey returns the accounts public key in compressed byte format
func (acc *Account) PublicKey() []byte {
	return acc.privKey.PubKey().Serialize()
}

// PrivateKey returns the accounts private key in compressed byte format
func (acc *Account) PrivateKey() *crypto.PrivateKey {
	return acc.privKey
}

func (acc *Account) saveWithPassphrase(passphrase string) error {
	savePrivateKeyWithPassphrase(acc.privKey, passphrase, acc.path)
	return nil
}

// UnlockWithPassphrase unlocks an account and generate its private key
func (acc *Account) UnlockWithPassphrase(passphrase string) error {
	privateKeyBytes, err := unlockPrivateKeyWithPassphrase(acc.path, passphrase)
	if err != nil {
		return err
	}
	if acc.privKey == nil {
		acc.privKey = &crypto.PrivateKey{}
	}
	acc.privKey, _, err = crypto.KeyPairFromBytes(privateKeyBytes)
	if err != nil {
		return err
	}
	addr, err := btypes.NewAddressFromPubKey(acc.privKey.PubKey())
	if err != nil {
		return err
	}
	if hex.EncodeToString(addr.ScriptAddress()) != acc.addr {
		return fmt.Errorf("Private key doesn't match address, the keystore file may be broken")
	}
	acc.unlocked = true
	return nil
}

var _ crypto.Signer = (*Account)(nil)

// Sign calculates an ECDSA signature of messageHash using privateKey.
// returns error if account is locked or sign process failed
func (acc *Account) Sign(messageHash *crypto.HashType) (*crypto.Signature, error) {
	if acc.unlocked == false || acc.privKey == nil {
		return nil, fmt.Errorf("Address unlocked")
	}
	return crypto.Sign(acc.privKey, messageHash)
}

// ReadPassphraseStdin reads passphrase from stdin without echo passphrase
// into terminal
func ReadPassphraseStdin() (string, error) {
	fmt.Println("Please Input Your Passphrase")
	input, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	passphrase := string(input)
	return passphrase, nil
}
