package api

import (
	"encoding/hex"
	"kortho/block"
	"kortho/blockchain"
	"kortho/transaction"
)

var blockChian blockchain.Blockchains

//Transaction 交易的数据结构
type Transaction struct {
	Nonce       uint64 `json:"nonce"`
	BlockNumber uint64 `json:"blocknumber"`
	Amount      uint64 `json:"amount"`
	From        string `json:"from"`
	To          string `json:"to"`
	Hash        string `json:"hash"`
	Signature   string `json:"signature"`
	Time        int64  `json:"time"`
	Script      string `json:"script"`
	Ord         *Order `json:"ord"`
	Tag         int32  `json:"tag"`
	KtoNum      uint64 `json:"ktonum"`
	PckNum      uint64 `json:"pcknum"`
}

//Block 块的数据结构
type Block struct {
	Version       uint64        `json:"version"`
	Height        uint64        `json:"height"`
	PrevBlockHash string        `json:"prevblockhash"`
	Hash          string        `json:"hash"`
	Root          string        `json:"root"`
	Timestamp     int64         `json:"timestamp"`
	Miner         string        `json:"miner"`
	Txs           []Transaction `json:"txs"`
}

//Order 订单数据结构
type Order struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	Price      uint64 `json:"price"`
	Hash       string `json:"hash"`
	Signature  string `json:"signature"`
	Ciphertext string `json:"ciphertext"`
	Tradename  string `json:"tradename"`
	Region     string `json:"region"`
}

func changeTransaction(tx *transaction.Transaction) (result Transaction) {
	result.Hash = hex.EncodeToString(tx.Hash)
	result.From = tx.From.String()
	result.Amount = tx.Amount
	result.Nonce = tx.Nonce
	result.To = tx.To.String()
	result.Signature = hex.EncodeToString(tx.Signature)
	result.Time = tx.Time
	result.BlockNumber = tx.BlockNumber
	result.Script = tx.Script
	result.Tag = tx.Tag
	result.KtoNum = tx.KtoNum
	result.PckNum = tx.PckNum

	if tx.IsOrderTransaction() {
		result.Ord.ID = string(tx.Order.ID)
		result.Ord.Hash = hex.EncodeToString(tx.Order.Hash)
		result.Ord.Signature = hex.EncodeToString(tx.Order.Signature)
		result.Ord.Ciphertext = hex.EncodeToString(tx.Order.Ciphertext)
		result.Ord.Address = string(tx.Order.Address.Bytes())
		result.Ord.Price = tx.Order.Price
		result.Ord.Tradename = tx.Order.Tradename
		result.Ord.Region = tx.Order.Region
	}

	return
}

func changeBlock(b *block.Block) (result Block) {
	result.Height = b.Height
	result.Hash = hex.EncodeToString(b.Hash)
	result.PrevBlockHash = hex.EncodeToString(b.PrevHash)
	result.Root = hex.EncodeToString(b.Root)
	result.Timestamp = b.Timestamp
	result.Version = b.Version
	result.Miner = b.Miner.String()

	for _, tx := range b.Transactions {
		result.Txs = append(result.Txs, changeTransaction(tx))
	}
	return
}
