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
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"golang.org/x/crypto/ssh/terminal"
)

// Manager is a directory based type to manipulate account
// Operation add/delete/query, import/export and sign are supported
type Manager struct {
	path     string
	accounts map[string]*acc.Account
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
	accounts := make([]*acc.Account, 0)
	for _, filePath := range files {
		account, err := acc.NewAccountFromFile(filePath)
		if err == nil {
			accounts = append(accounts, account)
		}
	}
	wlt.accounts = make(map[string]*acc.Account)
	for _, account := range accounts {
		wlt.accounts[account.Addr()] = account
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
func (wlt *Manager) ListAccounts() []*acc.Account {
	accounts := make([]*acc.Account, len(wlt.accounts))
	i := 0
	for _, acc := range wlt.accounts {
		accounts[i] = acc
		i++
	}
	return accounts
}

// NewAccount creates a ecdsa key pair and store them in a file encrypted
// by the passphrase user entered
// returns a hexstring format public key hash, address and error
func (wlt *Manager) NewAccount(passphrase string) (string, error) {
	privateKey, _, err := crypto.NewKeyPair()
	if err != nil {
		return "", err
	}
	return wlt.NewAccountWithPrivKey(privateKey, passphrase)
}

// NewAccountWithPrivKey store the give private key in a file encrypted
// by the passphrase user entered
// returns a hexstring format public key hash, address and error
func (wlt *Manager) NewAccountWithPrivKey(privKey *crypto.PrivateKey, passphrase string) (string, error) {
	address, err := btypes.NewAddressFromPubKey(privKey.PubKey())
	if err != nil {
		return "", err
	}
	account := &acc.Account{
		Path:     path.Join(wlt.path, fmt.Sprintf("%s.keystore", address.String())),
		PrivKey:  privKey,
		Address:  address,
		Unlocked: true,
	}
	if err := account.SaveWithPassphrase(passphrase); err != nil {
		return "", err
	}
	return address.String(), nil
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
	return hex.EncodeToString(acc.PrivKey.Serialize()), nil
}

// GetAccount checks if this Manager contains this public key
// and returns the related account if it exists
func (wlt *Manager) GetAccount(pubKeyHash string) (account *acc.Account, exist bool) {
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

	sig, err := crypto.Sign(account.PrivKey, hash)
	if err != nil {
		return nil, err
	}
	return sig.Serialize(), nil
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
