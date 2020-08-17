// Package blockchain 定义blockchain的接口，并实现了其对象
package blockchain

import (
	"kortho/block"
	"kortho/transaction"
	"kortho/types"
)

//Blockchains blockchain的接口规范
type Blockchains interface {
	NewBlock([]*transaction.Transaction, types.Address, types.Address, types.Address, types.Address) (*block.Block, error)
	AddBlock(*block.Block, []byte) error

	GetNonce([]byte) (uint64, error)
	GetBalance([]byte) (uint64, error)
	GetHeight() (uint64, error)
	GetHash(uint64) ([]byte, error)
	GetBlockByHash([]byte) (*block.Block, error)
	GetBlockByHeight(uint64) (*block.Block, error)
	GetFreezeBalance(address []byte) (uint64, error)

	GetTransactions(int64, int64) ([]*transaction.Transaction, error)
	GetTransactionByHash([]byte) (*transaction.Transaction, error)
	GetTransactionByAddr([]byte, int64, int64) ([]*transaction.Transaction, error)
	GetMaxBlockHeight() (uint64, error)

	GetTokenRoot(address, script string) ([]byte, error)
	GetTokenBalance(address, symbol []byte) (uint64, error)

	CalculationResults(block *block.Block) ([]byte, error)
	CheckResults(block *block.Block, resultHash, Ds, Cm, qtj []byte) bool
	GetBlockSection(lowH, heiH uint64) ([]*block.Block, error)
}
