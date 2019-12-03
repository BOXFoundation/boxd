// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utilcmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/util"
	"github.com/btcsuite/btcutil/base58"
	libcrypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "util [command]",
	Short: "some useful gadgets",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

var (
	inFilePath  string
	outFilePath string
)

// Init adds the sub command to the root command.
func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "encode [base58/base64][data]",
			Short: "convert data from hex format to base58/base64 format",
			Run:   encode,
		},
		&cobra.Command{
			Use:   "decode [base58/base64][data]",
			Short: "convert data from base58/base64 format to hex format",
			Run:   decode,
		},
		&cobra.Command{
			Use:   "convertaddress [address]",
			Short: "convert address to a special format",
			Run:   convertAddress,
		},
		&cobra.Command{
			Use:   "makesplitaddress [tx_hash] [preOutPoint_index] [(addr1, weight1), (addr2, weight2), (addr3, weight3), ...]",
			Short: "creat split address",
			Run:   makeSplitAddress,
		},
		&cobra.Command{
			Use:   "makecontractaddress [fromaddress] [nonce]",
			Short: "creat contract address",
			Run:   makeContractAddress,
		},
		&cobra.Command{
			Use:   "makep2pkhaddr [pubkey]",
			Short: "creat p2pKH address",
			Run:   makeP2PKHAddress,
		},
		&cobra.Command{
			Use:   "test [pubkey]",
			Short: "test",
			Run:   test,
		},
	)
	var (
		showPeerIDCmd = &cobra.Command{
			Use:   "showpeerid [optional|peerkey]",
			Short: "conversion peer key to peer ID",
			Run:   showPeerID,
		}
		createPeerIDCmd = &cobra.Command{
			Use:   "createpeerid ",
			Short: "create peer key",
			Run:   createPeerID,
		}
	)
	rootCmd.AddCommand(showPeerIDCmd, createPeerIDCmd)
	showPeerIDCmd.PersistentFlags().StringVar(&inFilePath, "i", "", "input file path, default nil")
	createPeerIDCmd.PersistentFlags().StringVar(&outFilePath, "o", ".", "output file path, default current directory")
}

func test(cmd *cobra.Command, args []string) {
	util.Isfile(args[0])
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println(cmd.Use)
		return
	}
	dataBytes, err := hex.DecodeString(args[1])
	if err != nil {
		fmt.Println("hex format data is needed:", args[1])
		return
	}
	switch args[0] {
	case "base58":
		data58 := base58.Encode(dataBytes)
		fmt.Println(data58)
	case "base64":
		data64 := base64.StdEncoding.EncodeToString(dataBytes)
		fmt.Println(data64)
	default:
		fmt.Println(cmd.Use)
	}
}

func decode(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println(cmd.Use)
		return
	}
	switch args[0] {
	case "base58":
		data58 := base58.Decode(args[1])
		fmt.Println(hex.EncodeToString(data58))
	case "base64":
		data64, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			fmt.Printf("decode %s error: %s\n", args[1], err)
			return
		}
		fmt.Println(hex.EncodeToString(data64))
	default:
		fmt.Println(cmd.Use)
	}

}

func convertAddress(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	dataBytes, err := hex.DecodeString(args[0])
	if err != nil {
		address, err := types.ParseAddress(args[0])
		if err != nil {
			fmt.Printf("invaild address: %s\n", err)
			return
		}
		fmt.Printf("address hash: %x\n", address.Hash160()[:])
		return
	}
	fmt.Println("the address hash could be generated to one of three addresses user friendly:")
	pubkeyAddr, err := types.NewAddressPubKeyHash(dataBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("P2PKH address:", pubkeyAddr)
	splitAddr, err := types.NewSplitAddressFromHash(dataBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("split address:", splitAddr)
	contractAddr, err := types.NewContractAddressFromHash(dataBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("contract address:", contractAddr)

}

func makeSplitAddress(cmd *cobra.Command, args []string) {
	if len(args) < 4 {
		fmt.Println(cmd.Use)
		return
	}
	txHash := new(crypto.HashType)
	if err := txHash.SetString(args[0]); err != nil {
		fmt.Printf("%s is invaild hash", args[0])
		return
	}
	index, err := strconv.ParseUint(args[1], 20, 64)
	if err != nil || index >= math.MaxUint32 {
		fmt.Printf("Invalid amount %s\n", args[1])
		return
	}
	weights := make([]uint32, 0)
	addrHashes := make([]*types.AddressHash, 0)
	for i := 2; i < len(args)-1; i += 2 {
		if address, err := types.ParseAddress(args[i]); err == nil {
			_, ok1 := address.(*types.AddressPubKeyHash)
			_, ok2 := address.(*types.AddressTypeSplit)
			if !ok1 && !ok2 {
				fmt.Printf("invaild address for %s, err: %s\n", args[i], err)
				return
			}
			addrHashes = append(addrHashes, address.Hash160())
		} else {
			fmt.Println(err)
			return
		}
		a, err := strconv.ParseUint(args[i+1], 10, 64)
		if err != nil || a >= math.MaxUint32 {
			fmt.Printf("get index %s, err: %s\n", args[i+1], err.Error())
			return
		}
		weights = append(weights, uint32(a))
	}
	if len(addrHashes) != len(weights) {
		fmt.Println("the length of addresses must be equal to the length of weights")
		return
	}
	splitAdd := txlogic.MakeSplitAddress(txHash, uint32(index), addrHashes, weights)
	fmt.Printf("address: %s, address hash: %x\n", splitAdd, splitAdd.Hash160()[:])
}

func makeContractAddress(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println(cmd.Use)
		return
	}
	addr, err := types.NewAddress(args[0])
	if err != nil {
		fmt.Printf("invails address: %s\n", args[0])
		return
	}
	nonce, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("invaild nonce:", err)
		return
	}
	contractAddress, err := types.MakeContractAddress(addr, nonce)
	if err != nil {
		fmt.Println("fail to make contract address:", err)
		return
	}
	fmt.Printf("address: %s, address hash: %x\n", contractAddress, contractAddress.Hash160()[:])
}

func makeP2PKHAddress(cmd *cobra.Command, args []string) {
	data, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println("hex format data is needed:", args[0])
		return
	}
	pubkey, err := crypto.PublicKeyFromBytes(data)
	if err != nil {
		fmt.Printf("%s generate public key failed: %s\n", args[0], err)
		return
	}
	addr, err := types.NewAddressFromPubKey(pubkey)
	if err != nil {
		fmt.Println("fail to generate a new p2pKHAddress:", err)
		return
	}
	fmt.Printf("address: %s, address hash: %x\n", addr, addr.Hash160()[:])
}

func showPeerID(cmd *cobra.Command, args []string) {
	var data string
	switch len(args) {
	case 0:
		if len(inFilePath) == 0 {
			fmt.Println("please input the path of peer.key")
			return
		}
		if _, err := os.Stat(inFilePath); err != nil {
			fmt.Println(err)
			return
		}
		dataByte, err := ioutil.ReadFile(inFilePath)
		if err != nil {
			fmt.Printf("read data from %s error: %s\n", args[0], err)
			return
		}
		data = string(dataByte)
	case 1:
		if _, err := libcrypto.ConfigDecodeKey(args[0]); err != nil {
			fmt.Printf("read data from %s, error: %s.", args[0], err)
			return
		}
		data = args[0]
	default:
		fmt.Println(cmd.Use)
		return
	}
	decodeData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		fmt.Printf("decode the string in %s error: %s\n", args[0], err)
		return
	}
	key, err := libcrypto.UnmarshalPrivateKey(decodeData)
	if err != nil {
		fmt.Println("conversion to private key error:", err)
		return
	}
	peerID, err := peer.IDFromPublicKey(key.GetPublic())
	if err != nil {
		fmt.Println("get peer ID from public key error:", err)
		return
	}
	fmt.Println("peer ID:", peerID.Pretty())
}

func createPeerID(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println(cmd.Use)
		return
	}
	var (
		peerKeyPath string
		defaultFile = outFilePath + "/peer.key"
	)
	_, err := os.Stat(outFilePath)
	if err != nil {
		if _, err := os.Stat(filepath.Dir(outFilePath)); err == nil {
			peerKeyPath = outFilePath
		} else {
			fmt.Printf("%s doesn't exist.\n", outFilePath)
			return
		}
	} else {
		if !util.Isfile(outFilePath) {
			if _, err := os.Stat(defaultFile); err != nil {
				peerKeyPath = defaultFile
			} else {
				fmt.Printf("%s has already existed.\n", defaultFile)
				return
			}
		} else {
			fmt.Printf("%s has already existed.\n", outFilePath)
			return
		}
	}
	key, _, err := libcrypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		fmt.Println("generate private key error:", err)
		return
	}
	data, err := libcrypto.MarshalPrivateKey(key)
	if err != nil {
		fmt.Println(err)
		return
	}
	b64data := base64.StdEncoding.EncodeToString(data)
	err = ioutil.WriteFile(peerKeyPath, []byte(b64data), 0400)
	if err != nil {
		fmt.Println("write data to file error:", err)
		return
	}
	fmt.Println("generate peer key:", peerKeyPath)
	peerID, err := peer.IDFromPublicKey(key.GetPublic())
	if err != nil {
		fmt.Println("get peer ID from public key error:", err)
		return
	}
	fmt.Println("peer ID:", peerID.Pretty())
}
