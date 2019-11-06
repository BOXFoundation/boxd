// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package root

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"

	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/util"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//ConnAddrFile is the default address of conn
const ConnAddrFile = ".cmd/connAddr"

// DefaultGRPCPort is the default listen port for box gRPC service.
const DefaultGRPCPort = 19191

// DefaultRPCHTTPPort is the default listen port for box RPC http service.
const DefaultRPCHTTPPort = 19190

// root command
var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "box",
	Short: "BOX Payout command-line interface",
	Long: `BOX Payout, a lightweight blockchain built for processing
			multi-party payments on digital content apps.`,
	Example: `
1.commands about wallet
  ./box wallet [command]
2. commands about contract
  ./box contract [command]
3.commands about transaction
  ./box tx [command]
4. commands about net and block_info
  ./box block information
	`,
	Version: fmt.Sprintf("%s %s(%s) %s\n", config.Version, config.GitCommit, config.GitBranch, config.GoVersion),
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

var logger = log.NewLogger("cmd")

// init sets flags appropriately.
func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.box.yaml)")

	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")

	RootCmd.PersistentFlags().StringP("network", "n", "mainnet", "network name (default mainnet)")
	viper.BindPFlag("network", RootCmd.PersistentFlags().Lookup("network"))

	RootCmd.PersistentFlags().String("workspace", "", "work directory for boxd (default ~/.boxd)")
	viper.BindPFlag("workspace", RootCmd.PersistentFlags().Lookup("workspace"))

	RootCmd.PersistentFlags().String("log-level", "error", "log level [debug|info|warn|error|fatal]")
	viper.BindPFlag("log.level", RootCmd.PersistentFlags().Lookup("log-level"))

	RootCmd.PersistentFlags().IP("rpc-addr", net.ParseIP("127.0.0.1"), "gRPC listen address.")
	viper.BindPFlag("rpc.address", RootCmd.PersistentFlags().Lookup("rpc-addr"))

	RootCmd.PersistentFlags().Uint("rpc-port", DefaultGRPCPort, "gRPC listen port.")
	viper.BindPFlag("rpc.port", RootCmd.PersistentFlags().Lookup("rpc-port"))

	RootCmd.PersistentFlags().IP("http-addr", net.ParseIP("127.0.0.1"), "rpc http listen address.")
	viper.BindPFlag("http.address", RootCmd.PersistentFlags().Lookup("http-addr"))

	RootCmd.PersistentFlags().Uint("http-port", DefaultRPCHTTPPort, "rpc http listen port.")
	viper.BindPFlag("http.port", RootCmd.PersistentFlags().Lookup("http-port"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	logger.SetLogLevel(viper.GetString("log.level"))

	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory or current directory with name ".box" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".box")
	}

	viper.SetEnvPrefix("box")
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "."))
	viper.AutomaticEnv() // read in environment variables that match

	viper.SetDefault("workspace", path.Join(home, ".boxd"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logger.Infof("Using config file: %s", viper.ConfigFileUsed())
	}
}

func init() {
	RootCmd.AddCommand(
		&cobra.Command{
			Use:   "setconnaddr [connAddr]",
			Short: "set default connAddr",
			Run:   setConnAddr,
		},
		&cobra.Command{
			Use:   "reset ",
			Short: "reset default connAddr",
			Run:   reset,
		},
	)
}

func setConnAddr(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	_, _, err := net.SplitHostPort(args[0])
	if err != nil {
		fmt.Printf("split %s error: %s", args[0], err)
	}
	// write the conn address to file
	if err := ioutil.WriteFile(ConnAddrFile, []byte(args[0]), 0644); err != nil {
		panic(err)
	}
}

func reset(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println(cmd.Use)
		return
	}
	if err := util.FileExists(ConnAddrFile); err != nil {
		return
	}
	if err := os.Remove(ConnAddrFile); err != nil {
		panic(err)
	}
}
