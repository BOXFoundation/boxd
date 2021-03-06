// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
)

// error
var (
	//blockchain.go
	ErrBlockExists                  = errors.New("Block already exists")
	ErrInvalidTime                  = errors.New("Invalid time")
	ErrTimeTooNew                   = errors.New("Block time too new")
	ErrNoTransactions               = errors.New("Block does not contain any transaction")
	ErrBlockTooBig                  = errors.New("Block too big")
	ErrFirstTxNotCoinbase           = errors.New("First transaction in block is not a coinbase")
	ErrMultipleCoinbases            = errors.New("Block contains multiple coinbase transactions")
	ErrBadMerkleRoot                = errors.New("Merkel root mismatch")
	ErrDuplicateTx                  = errors.New("Duplicate transactions in a block")
	ErrTooManySigOps                = errors.New("Too many signature operations in a block")
	ErrBadFees                      = errors.New("total fees for block overflows accumulator")
	ErrBadCoinbaseValue             = errors.New("Coinbase pays not equal to expected value")
	ErrUnfinalizedTx                = errors.New("Transaction has not been finalized")
	ErrWrongBlockHeight             = errors.New("Wrong block height")
	ErrFailedToVerifyWithConsensus  = errors.New("Failed to verify block with consensus")
	ErrBlockIsNil                   = errors.New("Block is nil")
	ErrOrphanBlockExists            = errors.New("Orphan block already exists")
	ErrFailedToSetEternal           = errors.New("Failed to set eternal block")
	ErrTokenInputsOutputNotEqual    = errors.New("Tx input tokens and output tokens unequal")
	ErrTokenInvalidName             = errors.New("Token name cannot be box")
	ErrParentBlockNotExist          = errors.New("Parent block does not exist")
	ErrBlockTimeOut                 = errors.New("The block is timeout")
	ErrFutureBlock                  = errors.New("Received a future block")
	ErrRepeatedMintAtSameTime       = errors.New("Repeated mint at same time")
	ErrFailedToVerifyWithCandidates = errors.New("Failed to verify block with candidates")
	ErrExpiredBlock                 = errors.New("Expired block")
	ErrBlockInSideChain             = errors.New("The block is in side chain")
	ErrInvalidInternalTxs           = errors.New("Invalid internal txs")
	ErrInvalidMessageSender         = errors.New("Invalid message sender")
	ErrContractNotFound             = errors.New("Contract not found")
	ErrMaxTxsSizeExceeded           = errors.New("Max txs size exceeded")
	ErrMaxCodeSizeExceeded          = errors.New("Max contract code size exceeded")
	ErrNonceTooLow                  = errors.New("Nonce is too low")
	ErrNonceTooBig                  = errors.New("Nonce is too big")
	ErrNonceExists                  = errors.New("Nonce already exists")
	ErrMixedVoutTx                  = errors.New("mixed vout transaction")

	//transaciton_pool.go
	ErrDuplicateTxInPool          = errors.New("Duplicate transactions in tx pool")
	ErrDuplicateTxInOrphanPool    = errors.New("Duplicate transactions in orphan pool")
	ErrCoinbaseTx                 = errors.New("Transaction must not be a coinbase transaction")
	ErrNonStandardTransaction     = errors.New("Transaction is not a standard transaction")
	ErrOutPutAlreadySpent         = errors.New("Output already spent by transaction in the pool")
	ErrOrphanTransaction          = errors.New("Orphan transaction cannot be admitted into the pool")
	ErrNonLocalMessage            = errors.New("Received non-local message")
	ErrLocalMessageNotChainUpdate = errors.New("Received local message is not a chain update")
	ErrUtxosOob                   = errors.New("Utxos in tx out of bound")
	ErrVoutsOob                   = errors.New("Vout in tx out of bound")

	//block.go
	ErrSerializeHeader                = errors.New("Serialize block header error")
	ErrEmptyProtoMessage              = errors.New("Empty proto message")
	ErrInvalidBlockHeaderProtoMessage = errors.New("Invalid block header proto message")
	ErrInvalidBlockProtoMessage       = errors.New("Invalid block proto message")
	ErrOutOfBlockGasLimit             = errors.New("Out of block gas limit")

	//trie.go
	ErrInvalidTrieProtoMessage = errors.New("Invalid trie proto message")
	ErrNodeNotFound            = errors.New("Node is not found")
	ErrInvalidNodeType         = errors.New("Invalid node type")
	ErrInvalidKeyPath          = errors.New("Invalid key path")

	//Receipt
	ErrInvalidReceiptProtoMessage = errors.New("Invalid receipt proto message")

	//Log
	ErrInvalidLogProtoMessage = errors.New("Invalid log proto message")

	//transaction.go
	ErrSerializeOutPoint                   = errors.New("Serialize outPoint error")
	ErrInvalidOutPointProtoMessage         = errors.New("Invalid OutPoint proto message")
	ErrInvalidTxInProtoMessage             = errors.New("Invalid TxIn proto message")
	ErrInvalidTxOutProtoMessage            = errors.New("Invalid TxOut proto message")
	ErrInvalidTxProtoMessage               = errors.New("Invalid tx proto message")
	ErrInvalidIrreversibleInfoProtoMessage = errors.New("Invalid IrreversibleInfo proto message")
	ErrInvalidFee                          = errors.New("Invalid transaction fee")
	ErrContractDataNotFound                = errors.New("Contract data not found in tx")
	ErrMultipleContractVouts               = errors.New("Multiple contract vouts")

	//address.go
	ErrInvalidPKHash        = errors.New("PkHash must be 20 bytes")
	ErrInvalidAddressString = errors.New("Invalid box address format")

	//utils.go
	ErrNoTxInputs           = errors.New("Transaction has no inputs")
	ErrNoTxOutputs          = errors.New("Transaction has no outputs")
	ErrBadTxOutValue        = errors.New("Invalid output value")
	ErrDoubleSpendTx        = errors.New("Transaction must not use any of the same outputs as other transactions")
	ErrBadCoinbaseScriptLen = errors.New("Coinbase scriptSig out of range")
	ErrBadTxInput           = errors.New("Transaction input refers to null out point")
	ErrMissingTxOut         = errors.New("Referenced utxo does not exist")
	ErrImmatureSpend        = errors.New("Attempting to spend an immature coinbase")
	ErrSpendTooHigh         = errors.New("Transaction is attempting to spend more value than the sum of all of its inputs")
	ErrMultipleOpReturnOuts = errors.New("Transaction must not contain multiple OPRETURN tx outs")

	//utxoset.go
	ErrTxOutIndexOob               = errors.New("Transaction output index out of bound")
	ErrUtxoNotFound                = errors.New("utxo not found")
	ErrAddExistingUtxo             = errors.New("Trying to add utxo already existed")
	ErrInvalidUtxoWrapProtoMessage = errors.New("Invalid utxo wrap proto message")

	//filterholder.go
	ErrInvalidFilterHeight = errors.New("Filter can only be added in chain sequence")
	ErrLoadBlockFilters    = errors.New("Fail to load block filters")

	//sectionmanager.go
	ErrBloomBitOutOfBounds = errors.New("Bloom bit out of bounds")
	ErrInvalidBounds       = errors.New("Invalid section bounds")

	EvilBehavior = []error{
		ErrInvalidTime, ErrNoTransactions, ErrBlockTooBig,
		ErrFirstTxNotCoinbase, ErrMultipleCoinbases, ErrBadMerkleRoot,
		ErrDuplicateTx, ErrTooManySigOps, ErrBadFees,
		ErrBadCoinbaseValue, ErrUnfinalizedTx, ErrWrongBlockHeight,
		ErrDuplicateTxInPool, ErrDuplicateTxInOrphanPool, ErrCoinbaseTx,
		ErrNonStandardTransaction, ErrOutPutAlreadySpent, ErrOrphanTransaction,
		ErrDoubleSpendTx, ErrRepeatedMintAtSameTime,
	}
)
