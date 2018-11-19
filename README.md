# Boxd

A Go implementation of BOX Payout blockchain. 

## Overview

Content Box is a platform which was developed for building digital ecosystems. Boxd is a backend for Content Box blockchain platform.

In this guide we will deploy Boxd Blockchain Platform based on three kinds of nodes with the same OS on the private net.

As a node OS we will use:

* Linux

* Mac OSX

> The Windows version is coming soon.

## Prerequisite

|Components|Version	| Description |
|:---|:---:|:---:|
|Golang| >= 1.11| -- |
|Govendor| -| A dependency management tool for Go. |
|Rocksdb| >= 5.0.1| A C++ library providing an embedded key-value store. |

We cannot use our Homebrew tap to install Boxd yet. But we will fix it in the future.

### Preparing environment

* OS X:

	1. Install Go compiler:
	
		```
		brew install go
	
	2. Install rocksdb:
	
		```
		brew install rocksdb
		```
	
	3. Use govendor:
	
		```
		go get -u github.com/kardianos/govendor
		```

* Linux:

	Click [here](https://golang.org/doc/install) to install Go compiler.
	
	1. Install rocksdb:
		#### Ubuntu
		```
		apt-get update
	apt-get -y install build-essential libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev
	git clone https://github.com/facebook/rocksdb.git
	cd rocksdb && make shared_lib && make install-shared
		```
		#### CentOS
		```
		yum -y install epel-release && yum -y update
	yum -y install gflags-devel snappy-devel zlib-devel bzip2-devel gcc-c++  libstdc++-devel
		git clone https://github.com/facebook/rocksdb.git
	cd rocksdb && make shared_lib && make install-shared
		```
	
	2. Use govendor:
	
		```
		go get -u github.com/kardianos/govendor
		```
	
### Building
1. Run the following command to obtain boxd and all dependencies:

	```
	go get github.com/BOXFoundation/boxd
	go mod vendor
	```
	> boxd (and utilities) will now be installed in $GOPATH/bin. If you did not already add the bin directory to your system path during Go installation, we recommend you do so now.
	
2. Once the dependencies are installed, run

	```
	make
	```

## Getting Started

The box chain you are running at this point is private and is different from the official Testnet and Mainnet.

### Configuration

Boxd has several configuration options available to tweak how it runs, but all of the basic operations described in the intro section work with a little configuration.

The following is the template for the overall configuration.

	
	network: testnet
	workspace: .devconfig/ws1
	database:
	    name: rocksdb
	# Log Configuration
	log:
	    out:
	    	 # [stdout, stderr, null]
	        name: stderr 
	    # Log level [debug, info, warning, error, fatal]
	    level: debug 
	    # Log Format [json, text]
	    formatter:
	        name: text
	    hooks:
	        - name: filewithformatter
	          options:
	              filename: box.log
	              maxlines: 100000
	              # daily: true
	              # maxsize: 10240000
	              rotate: true
	              # [0:panic, 1:fatal, 2:error, 3:warning, 4:info, 5:debug]
	              level:  4 
	# P2p network Configuration
	p2p:
	    key_path: peer.key
	    port: 19199
	    # The first node to the network needn't to have the seeds.
	    # Otherwise, every nodes of the network need to config the seeds to join into the network.
	    seeds:
	        - "/ip4/127.0.0.1/tcp/19199/p2p/12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza"
	    bucket_size: 16
	    latency: 10
	    conn_max_capacity: 200
	    conn_load_factor: 0.8
	rpc:
	    port: 19191
	    http:
	        port: 19190
	#Dpos Configuration
	dpos:
		 # Store the Private key
	    keypath: key.keystore
	    # Distinguish whether it is a miner
	    enable_mint: false
	    passphrase: 1
	# Burying point Configuration
	metrics:
		 # If true, you need to install Influxdb on your host node
	    enable: false
	    # Following are Influxdb configuration
	    host: http://localhost:8086
	    db: box
	    user: 
	    password:
	    tags: 

### Starting up your own node

#### Running seed node
Run one or more seed nodes on your networks through the above configuration, which all other nodes need to sync routing table upon. 

>Or if you want to use our official tesetnet seeds and connect in our testnet, skip this step.

	cd $GOPATH/src/github.com/BOXFoundation/boxd
	./box start --config=./.devconfig/.box-1.yaml

If it goes smoothly, you will see the following contents:

	[image link]

We will find this peer's Id in the second line. 

#### Running node
Edit your node's yaml and add seeds' link to it.

	p2p:
	    key_path: peer.key
	    port: 19189
	    seeds:
	        - "/ip4/127.0.0.1/tcp/19199/p2p/12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza"
	        - "/ip4/127.0.0.1/tcp/19199/p2p/12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza"
	    bucket_size: 16
	    latency: 10
	    conn_max_capacity: 200
	    conn_load_factor: 0.8

If you are a miner node, you need to change dpos.enable_mint to true.

Start the nodes with the following commands:

	
	cd $GOPATH/src/github.com/BOXFoundation/boxd
	./box start --config=./.devconfig/.box-1.yaml

## Docker
This way to create private chains can only be done by using docker and some configuration in the source code.

1. Pull from dockerhub directly
	
		docker pull jerrypeen/boxd:latest

2. Enter custom workspace and copy some configuration from the boxd source code
	
		cp -rf $GOPATH/src/github.com/BOXFoundation/boxd/docker/docker-compose.yml $GOPATH/src/github.com/BOXFoundation/boxd/docker/.devconfig $WORKSPACE/
		
3. Start image
	
		docker-compose up
		
> Docker will start six miners by default. Boxd suggest you to provide docker with at lest the following configuration
> 
> * cpus : 4
> * memory: 8.0 GiB
> * Swap: 2.0 Gib


	

# Contribution

Thank you very much for your thinking and help on Boxd. Even a small amount of help in code, community or documentation is making us better.

If you are willing to contribute to Boxd, please fork, fix, commit and send pull requests so that we can review the code and merge it into the main code base. If you have very complex or even structural changes, please contact our developers on the gitter channel to ensure that it fits the overall way of thinking of our code. Earlier communication will make you and us more efficient.

Your code needs to meet the following requirements:
1. Good golang code style.
2. Enough annotation.
3. Enough high coverage of test cases
4, Provide sufficient and detailed description in pull request.

# License

BOX Payout is released under the MIT License. Please refer to the [LICENSE](https://github.com/BOXFoundation/boxd/blob/master/LICENSE) file that accompanies this project for more information including complete terms and conditions.
