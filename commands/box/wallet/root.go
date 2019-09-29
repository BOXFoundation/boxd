// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/BOXFoundation/boxd/wallet/account"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var cfgFile string
var walletDir string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wallet",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	Example: `
	1.new account
	  ./box wallet newaccount
	2.list account in your wallet
	  ./box wallet listaccounts
	3.validate address
	  ./box wallet validateaddress  b1fBA6qDFe3oZKkuiVpdv77oSXNQocBJL3p
	4.sign transaction 
	  ./box --wallet_dir keyfile tx signrawtx b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m 12240a220a20206029513377e45fed5cf8486b76664864a8138d1cc5248c84202eab2c803b541a1d080a121976a9146aeacee33019c341b1700de7a0485ce13f3d02b988ac1a1d0814121976a914886d68724989517491e92c420a820a9e9d5b41d688ac1a2408d2f499ecb3aacf3a121976a914816666b318349468f8146e76e4e3751d937c14cb88ac
	5.get balance
	  ./box wallet getbalance b1YLUNwJD124sv9piRvkqcmfcujTZtHhHSz
	6.import [option|path_keystore]
	   ./box wallet integration_tests/.devconfig/keyfile/key3.keystore
	`,
}

// Init adds the sub command to the root command.
func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", common.DefaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		// &cobra.Command{
		// 	Use:   "encryptwallet [passphrase]",
		// 	Short: "Encrypt a wallet with a passphrase",
		// 	Run: func(cmd *cobra.Command, args []string) {
		// 		fmt.Println("encryptwallet called")
		// 	},
		// },
		// &cobra.Command{
		// 	Use:   "encryptwallet [passphrase]",
		// 	Short: "Encrypt a wallet with a passphrase",
		// 	Run: func(cmd *cobra.Command, args []string) {
		// 		fmt.Println("encryptwallet called")
		// 	},
		// },
		// &cobra.Command{
		// 	Use:   "lock [address]",
		// 	Short: "Lock a account",
		// 	Run:   lock,
		// },
		// &cobra.Command{
		// 	Use:   "unlock [address]",
		// 	Short: "unlock account",
		// 	Run:   unlock,
		// },
		&cobra.Command{
			Use:   "newaccount [account]",
			Short: "Create a new account",
			Run:   newAccountCmdFunc,
		},
		&cobra.Command{
			Use:   "dumpprivkey [address]",
			Short: "Dump private key for an address",
			Run:   dumpPrivKeyCmdFunc,
		},
		&cobra.Command{
			Use:   "dumpwallet [filename]",
			Short: "Dump wallet to a file",
			Run:   dumpwallet,
		},
		&cobra.Command{
			Use:   "getwalletinfo [address]",
			Short: "Get the basic informatio for a wallet",
			Run:   getwalletinfo,
		},
		&cobra.Command{
			Use:   "listaccounts",
			Short: "List local accounts",
			Run:   listAccountCmdFunc,
		},
		&cobra.Command{
			Use:   "decodeaddress [address]",
			Short: "decode address from base58 format to hash160",
			Run:   decodeAddress,
		},
		&cobra.Command{
			Use:   "getbalance [address]",
			Short: "Get the balance for any given address",
			Run:   getBalanceCmdFunc,
		},
		&cobra.Command{
			Use:   "signmessage [message] [optional publickey]",
			Short: "Sign a message with a publickey",
			Run:   signMessageCmdFunc,
		},
		&cobra.Command{
			Use:   "validateaddress [address]",
			Short: "Check if an address is valid",
			Run:   validateMessageCmdFunc,
		},
		&cobra.Command{
			Use:   "import [option|path_keytore]",
			Short: "Import a wallet from a file",
			Run:   importwallet,
		},
	)
}

func newAccountCmdFunc(cmd *cobra.Command, args []string) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	addr, err := wltMgr.NewAccount(passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Created new account. Address:%s\n", addr)
}

func listAccountCmdFunc(cmd *cobra.Command, args []string) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, acc := range wltMgr.ListAccounts() {
		fmt.Println("Managed Address:", acc.Addr(), "Public Key Hash:", hex.EncodeToString(acc.PubKeyHash()))
	}
}

func getwalletinfo(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}
	addr := args[0]
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	walletinfo, exist := wltMgr.GetAccount(addr)
	if exist {
		fmt.Println("path: ", walletinfo.Path)
		fmt.Println("Address: ", walletinfo.Addr())
		fmt.Printf("Pubkey Hash: %x \n", walletinfo.PubKeyHash())
		fmt.Println("UnLocked: ", walletinfo.Unlocked)
	}
}

func dumpPrivKeyCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}
	addr := args[0]
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	privateKey, err := wltMgr.DumpPrivKey(addr, passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Address: %s\nPrivate Key: %s\n", addr, privateKey)
}

func dumpwallet(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}

	if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
		fmt.Println("The file already exists and the command will overwrite the contents of the file.")

	}
	file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_TRUNC|os.O_APPEND|os.O_CREATE, 0600)
	defer file.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	file.Write([]byte("# Wallet dump created by ContentBox\n" + "# * Created on " + time.Now().Format("2006-01-02 15:04:05") + "\n"))

	addrs := make([]string, 0)
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, acc := range wltMgr.ListAccounts() {
		addrs = append(addrs, acc.Addr())
	}
	success := 0

	for _, x := range addrs {
		addr := x
		wltMgr, err := wallet.NewWalletManager(walletDir)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Print("addr: " + addr)
		for i := 3; i >= 1; i-- {
			fmt.Printf(" You have %d remaining input opportunities\n", i)
			passphrase, err := wallet.ReadPassphraseStdin()
			if err != nil {
				fmt.Println(err)
				return
			}
			privateKey, err := wltMgr.DumpPrivKey(addr, passphrase)
			if err != nil {
				fmt.Print("error:", err)
				if i == 1 {
					fmt.Println("This wallet dump failed ")

				}
				continue
			}
			fmt.Printf("Address: %s dump successful \n", addr)
			file.Write([]byte("privateKey: " + privateKey + " " + time.Now().Format("2006-01-02 15:04:05") + " # addr=" + addr + "\n"))
			success++
			break

		}
		fmt.Println()
	}
	fmt.Printf("All wallets are dumped. %d successful %d failed\n", success, len(addrs)-success)
}

func decodeAddress(cmd *cobra.Command, args []string) {
	if len(args) != 1 && len(args[0]) == 0 {
		fmt.Println("invalid args")
		fmt.Println(cmd.UsageString())
		return
	}
	address, err := types.ParseAddress(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("address hash: %x\n", address.Hash160()[:])
}

func getBalanceCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
	}
	addrs := make([]string, 0)
	if len(args) < 1 {
		wltMgr, err := wallet.NewWalletManager(walletDir)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, acc := range wltMgr.ListAccounts() {
			addrs = append(addrs, acc.Addr())
		}
	} else {
		addrs = args
	}
	if err := types.ValidateAddr(addrs...); err != nil {
		fmt.Println("Verification address failed:", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "GetBalance",
		&rpcpb.GetBalanceReq{Addrs: addrs}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetBalanceResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.GetBalanceResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	for i, b := range resp.Balances {
		fmt.Printf("%s: %d\n", addrs[i], b)
	}
}

func signMessageCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println(cmd.Use)
		return
	}
	msg, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, exists := wltMgr.GetAccount(args[1]); !exists {
		fmt.Println(args[1], " is not a managed account")
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	sig, err := wltMgr.Sign(msg, args[1], passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Signature:", hex.EncodeToString(sig))
}

func validateMessageCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}
	addr := new(types.AddressPubKeyHash)
	if err := addr.SetString(args[0]); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(args[0], " is a valid address")
	}
}

func importwallet(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fmt.Println(cmd.Use)
		return
	}
	if len(args) == 0 {
		addr, err := importPrivateKey()
		if err != nil {
			fmt.Println("import private key failed:", err)
			return
		}
		fmt.Printf("create new account. Address:%s\n", addr)
	} else {
		// Verify that the file exists
		keyFile := args[0]
		err := importKeyStore(keyFile)
		if err != nil {
			fmt.Println("import key store failed:", err)
			return
		}
	}
}

func readPrivateStdin() (string, error) {
	fmt.Println("Please Input Your PrivateKey:")
	input, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	privKeyString := string(input)
	return privKeyString, nil
}

func importPrivateKey() (string, error) {
	privKeyString, err := readPrivateStdin()
	if err != nil {
		fmt.Println("Read privkey failed:", err)
		return "", err
	}
	privKeyBytes, err := hex.DecodeString(privKeyString)
	if err != nil {
		fmt.Println("Invalid private key", err)
		return "", err
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	privKey, _, err := crypto.KeyPairFromBytes(privKeyBytes)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	addr, err := wltMgr.NewAccountWithPrivKey(privKey, passphrase)
	if err != nil {
		return "", err
	}
	return addr, nil
}

func importKeyStore(keyFile string) error {
	if _, err := os.Stat(keyFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("abi file is not exists")
		} else {
			fmt.Println("abi file status error:", err)
		}
		return err
	}
	keyStAdd, err := account.GetKeystoreAddress(keyFile)
	if err != nil {
		fmt.Println("get address in keystore failed:", err)
	}
	accI, err := account.NewAccountFromFile(keyFile)
	if err != nil {
		fmt.Printf("account for %s not initialled\n", keyStAdd)
		return err
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err := accI.UnlockWithPassphrase(passphrase); err != nil {
		fmt.Println("Fail to unlock account", err)
		return err
	}
	if err := util.MkDir(walletDir); err != nil {
		panic(err)
	}

	newFile := walletDir + "/" + filepath.Base(keyFile)
	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(newFile, data, 0644)
	if err != nil {
		panic(err)
	}
	return nil
}
