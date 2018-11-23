# Boxd

A Go implementation of BOX Payout blockchain.[![Build Status](https://travis-ci.com/BOXFoundation/boxd.svg?token=v6N8VybyjmC1GLSWZv92&branch=develop)](https://travis-ci.com/BOXFoundation/boxd)

## Overview

Content Box is a platform which was developed for building digital ecosystems. Boxd is a backend for Content Box blockchain platform.

In this guide, we will deploy boxd local regression testnet.

As a node OS we will use:

* Linux

* Mac OSX

> The Windows version is coming soon.

## Prerequisite

|Components|Version	| Description |
|:---|:---:|:---:|
|Golang| >= 1.11| -- |
|Govendor| -| A dependency management tool for Go. |
|Rocksdb| >= 5.0.1|  A high performance embedded database for key-value data. |

### Preparing environment

* OS X:

	1. Install Go:
	
		```
		brew install go
		```
	
	2. Install rocksdb:
	
		```
		brew install rocksdb
		```
	
	3. Install govendor:
	
		```
		go get -u github.com/kardianos/govendor
		```

* Linux:

	Click [here](https://golang.org/doc/install) to install Go.
	
	1. Install rocksdb:
		#### Ubuntu
			apt-get update
			apt-get -y install build-essential libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev
			git clone https://github.com/facebook/rocksdb.git
			cd rocksdb && make shared_lib && make install-shared
		#### CentOS
			yum -y install epel-release && yum -y update
			yum -y install gflags-devel snappy-devel zlib-devel bzip2-devel gcc-c++  libstdc++-devel
			git clone https://github.com/facebook/rocksdb.git
			cd rocksdb && make shared_lib && make install-shared
	
	2. Install govendor:
	
		```
		go get -u github.com/kardianos/govendor
		```
	
### Build
1. Run the following command to obtain boxd and all dependencies:

	```
	go get github.com/BOXFoundation/boxd
	go mod vendor
	```
	> boxd (and utilities) will now be installed in $GOPATH/bin. If you did not already add the bin directory to your system path during Go installation, we recommend you do so now.
	
2. Once the dependencies are installed, run:

	```
	make
	```

## Getting Started

The box chain you are running at this point is local is different from the official Testnet and Mainnet.

### Configuration

Boxd has several configuration options available to tweak how it runs, but all of the basic operations described in the intro section work with a little configuration.

The following is the template for the overall configuration.

	
	network: testnet
	workspace: .devconfig/ws1
	database:
	    name: rocksdb
	log:
	    level: debug 
	p2p:
	    key_path: peer.key
	    port: 19199
	    bucket_size: 16
	    latency: 10
	    conn_max_capacity: 200
	    conn_load_factor: 0.8
	rpc:
	    port: 19191
	    http:
	        port: 19190
	dpos:
		 # Store the Private key
	    keypath: key.keystore
	    enable_mint: false
	    passphrase: 1

### Starting up your own node

#### Running seed node
Run one or more seed nodes on your networks through the above configuration, which all other nodes need to sync routing table with. 

>Or if you want to use our official tesetnet seeds and connect in our testnet, skip this step.

	cd $GOPATH/src/github.com/BOXFoundation/boxd
	./box start --config=./.devconfig/.box-1.yaml

We will find this peer's Id in the second line of the log. 

#### Running node
Edit your node's yaml and add seeds' link to it.

	p2p:
	    key_path: peer.key
	    port: 19189
	    seeds:
	        - "/ip4/127.0.0.1/tcp/19199/p2p/12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza"
	    bucket_size: 16
	    latency: 10
	    conn_max_capacity: 200
	    conn_load_factor: 0.8

If you are a miner node, you need to change `dpos.enable_mint` to true.

Start the nodes with the following commands:

	
	cd $GOPATH/src/github.com/BOXFoundation/boxd
	./box start --config=./.devconfig/.box-1.yaml

## Docker

1. Pull from dockerhub directly.
	
		docker pull jerrypeen/boxd:latest

2. Enter custom workspace and copy some configuration from the boxd source code.
	
		cp -rf $GOPATH/src/github.com/BOXFoundation/boxd/docker/docker-compose.yml $GOPATH/src/github.com/BOXFoundation/boxd/docker/.devconfig $WORKSPACE/
		
3. Start container.
	
		docker-compose up
		
> Docker will start six miners by default. We suggest you to provide docker with at least the following configuration:
> 
> * cpus : 4
> * memory: 8.0 GiB
> * Swap: 2.0 Gib


	

# Contribution

Thank you very much for your considering helping Boxd. Even a small amount of help in code, community or documentation is making us better.

If you are willing to contribute to Boxd, please fork, fix, commit and send pull requests so that we can review the code and merge it into the main code base. If you have very complex or even structural changes, please contact our developers to ensure that it fits the overall way of thinking of our code. Earlier communication will make you and us more efficient.

Your code needs to meet the following requirements:
1. Good golang code style.
2. Enough annotation.
3. Enough high coverage of test cases
4, Provide sufficient and detailed description in pull request.

# License

BOX Payout is released under the MIT License. Please refer to the [LICENSE](https://github.com/BOXFoundation/boxd/blob/master/LICENSE) file that accompanies this project for more information including complete terms and conditions.
