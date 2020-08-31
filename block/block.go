// Package block block包定义了block的数据结构和一些方法
package block

import (
	"bytes"
	"encoding/json"

	"kortho/transaction"
	"kortho/types"
	"kortho/util/miscellaneous"

	"golang.org/x/crypto/sha3"
)

// Block 块数据结构
type Block struct {
	Height       uint64                     `json:"height"`    //当前块号
	PrevHash     []byte                     `json:"prevHash"`  //上一块的hash json:"prevBlockHash --> json:"prevHash
	Hash         []byte                     `json:"hash"`      //当前块hash
	Transactions []*transaction.Transaction `json:"txs"`       //交易数据
	Root         []byte                     `json:"root"`      //默克根
	Version      uint64                     `json:"version"`   //版本号
	Timestamp    int64                      `json:"timestamp"` //时间戳
	Miner        types.Address              `json:"miner"`     //矿工地址
}

func newBlock(height uint64, prevHash []byte, transactions []*transaction.Transaction) *Block {
	block := &Block{
		Height:       height,
		PrevHash:     prevHash,
		Transactions: transactions,
	}
	return block
}

// Serialize 使用json格式进行序列化
func (b *Block) Serialize() []byte {
	data, err := json.Marshal(b)
	if err != nil {
		return nil
	}
	return data
}

// Deserialize 对json格式的块数据反序列化
func Deserialize(data []byte) (*Block, error) {
	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

//SetHash 对块数据摘要出hash
func (b *Block) SetHash() {
	heightBytes := miscellaneous.E64func(b.Height)
	txsBytes, _ := json.Marshal(b.Transactions)
	timeBytes := miscellaneous.E64func(uint64(b.Timestamp))
	blockBytes := bytes.Join([][]byte{heightBytes, b.PrevHash, txsBytes, timeBytes}, []byte{})
	hash := sha3.Sum256(blockBytes)
	b.Hash = hash[:]
}
