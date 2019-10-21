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
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "util [command]",
	Short: "",
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
			Short: "convert to base58 format",
			Run:   encode,
		},
		&cobra.Command{
			Use:   "decode [base58/base64][data]",
			Short: "test util",
			Run:   decode,
		},
		&cobra.Command{
			Use:   "decodeaddress [address]",
			Short: "decode address from base58 format to hash160",
			Run:   decodeAddress,
		},
		&cobra.Command{
			Use:   "encodeaddress [address]",
			Short: "encode address hash160 to base58",
			Run:   encodeaddress,
		},
		&cobra.Command{
			Use:   "splitaddress [tx_hash] [(addr1, weight1), (addr2, weight2), (addr3, weight3), ...]",
			Short: "creat split address",
			Run:   splitaddress,
		},
	)
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Println(cmd.Use)
		return
	}
	dataBytes := []byte(args[0])
	if args[0] == "base58" {
		data58 := crypto.Base58CheckEncode(dataBytes)
		fmt.Println("base58 farmat:", data58)
	} else if args[0] == "base64" {
		data64 := base64.StdEncoding.EncodeToString(dataBytes)
		fmt.Println("base64 format:", data64)
	}
}

func decode(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Println(cmd.Use)
		return
	}
	if args[0] == "base58" {
		data58, err := crypto.Base58CheckDecode(args[1])
		if err != nil {
			fmt.Printf("decode %s err: %s\n", args[1], err)
			return
		}
		fmt.Println("decode base58:", string(data58))
	} else if args[0] == "base64" {
		data64, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			fmt.Printf("decode %s error: %s\n", args[1], err)
			return
		}
		fmt.Println(string(data64))
	}
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

func encodeaddress(cmd *cobra.Command, args []string) {
	dataBytes, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Printf("%s is not hex format data\n", args[0])
		return
	}
	fmt.Println("the address hash could be generated to one of three addresses user friendly:")
	pubkeyAddr, err := types.NewAddressPubKeyHash(dataBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("pubKey address:", pubkeyAddr)
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

func splitaddress(cmd *cobra.Command, args []string) {
	txHash := new(crypto.HashType)
	if err := txHash.SetString(args[0]); err != nil {
		fmt.Printf("%s is invaild hash", args[0])
		return
	}
	addrs, weights := make([]string, 0), make([]uint32, 0)
	for i := 1; i < len(args)-1; i += 2 {
		if address, err := types.ParseAddress(args[i]); err != nil {
			_, ok1 := address.(*types.AddressPubKeyHash)
			_, ok2 := address.(*types.AddressTypeSplit)
			if !ok1 && !ok2 {
				fmt.Printf("invaild address for %s, err: %s\n", args[i], err)
				return
			}
		}
		addrs = append(addrs, args[i])
		a, err := strconv.ParseUint(args[i+1], 10, 64)
		if err != nil || a >= math.MaxUint32 {
			fmt.Printf("Invalid amount %s\n", args[i+1])
			return
		}
		weights = append(weights, uint32(a))
	}
	addrHashes := make([]*types.AddressHash, 0, len(addrs))
	for _, addr := range addrs {
		address, _ := types.ParseAddress(addr)
		addrHashes = append(addrHashes, address.Hash160())
	}
	splitadd := txlogic.MakeSplitAddress(txHash, 0, addrHashes, weights)
	fmt.Println(splitadd)
}
