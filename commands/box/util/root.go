// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utilcmd

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/btcsuite/btcutil/base58"
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
			Run:   convertaddress,
		},
		&cobra.Command{
			Use:   "makesplitaddress [tx_hash] [(addr1, weight1), (addr2, weight2), (addr3, weight3), ...]",
			Short: "creat split address",
			Run:   makesplitaddress,
		},
		&cobra.Command{
			Use:   "makeContractAddr [fromaddress] [nonce]",
			Short: "creat contract address",
			Run:   makeContractAddr,
		},
		&cobra.Command{
			Use:   "makep2pKHAddr [pubkey]",
			Short: "creat p2pKH address",
			Run:   makep2pKHAddr,
		},
	)
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Println(cmd.Use)
		return
	}
	dataBytes, err := hex.DecodeString(args[1])
	if err != nil {
		fmt.Println("hex format data is needed:", args[1])
		return
	}
	if args[0] == "base58" {
		data58 := base58.Encode(dataBytes)
		fmt.Println(data58)
	} else if args[0] == "base64" {
		data64 := base64.StdEncoding.EncodeToString(dataBytes)
		fmt.Println(data64)
	}
}

func decode(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Println(cmd.Use)
		return
	}
	if args[0] == "base58" {
		data58 := base58.Decode(args[1])
		fmt.Println(hex.EncodeToString(data58))
	} else if args[0] == "base64" {
		data64, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			fmt.Printf("decode %s error: %s\n", args[1], err)
			return
		}
		fmt.Println(hex.EncodeToString(data64))
	}
}

func convertaddress(cmd *cobra.Command, args []string) {
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
	} else {
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
}

func makesplitaddress(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		fmt.Println(cmd.Use)
		return
	}
	txHash := new(crypto.HashType)
	if err := txHash.SetString(args[0]); err != nil {
		fmt.Printf("%s is invaild hash", args[0])
		return
	}
	addrs, weights := make([]string, 0), make([]uint32, 0)
	for i := 1; i < len(args)-1; i += 2 {
		if address, err := types.ParseAddress(args[i]); err == nil {
			_, ok1 := address.(*types.AddressPubKeyHash)
			_, ok2 := address.(*types.AddressTypeSplit)
			if !ok1 && !ok2 {
				fmt.Printf("invaild address for %s, err: %s\n", args[i], err)
				return
			}
		} else {
			fmt.Println(err)
			return
		}
		addrs = append(addrs, args[i])
		a, err := strconv.ParseUint(args[i+1], 10, 64)
		if err != nil || a >= math.MaxUint32 {
			fmt.Printf("Invalid amount %s\n", args[i+1])
			return
		}
		weights = append(weights, uint32(a))
		if len(addrs) != len(weights) {
			fmt.Println("the length of addresses must be equal to the length of weights")
			return
		}
	}
	addrHashes := make([]*types.AddressHash, 0, len(addrs))
	for _, addr := range addrs {
		address, _ := types.ParseAddress(addr)
		addrHashes = append(addrHashes, address.Hash160())
	}
	splitadd := txlogic.MakeSplitAddress(txHash, 0, addrHashes, weights)
	fmt.Println(splitadd)
}

func makeContractAddr(cmd *cobra.Command, args []string) {
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
	fmt.Println(contractAddress)
}

func makep2pKHAddr(cmd *cobra.Command, args []string) {
	data, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println("hex format data is needed:", args[0])
		return
	}
	pubkey, err := crypto.PublicKeyFromBytes(data)
	if err != nil {
		fmt.Printf("%s generate public key failed: %s", args[0], err)
		return
	}
	addr, err := types.NewAddressFromPubKey(pubkey)
	if err != nil {
		fmt.Println("fail to generate a new p2pKHAddress:", err)
		return
	}
	fmt.Println(addr)
}
