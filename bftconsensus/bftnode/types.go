//Package bftnode implements to package,check,delive and commit a block.
package bftnode

import (
	"kortho/bftconsensus/node"
	"kortho/blockchain"
	"kortho/config"
	p2pnode "kortho/p2p/node"
	"kortho/txpool"
)

//Node interface
type Node interface {
	//run a bft node and package a new block per second.
	Run()
	//add node into cluster.
	Add(string) error
	Remove(string) error
}

type bftnode struct {
	Bn         node.Node              //bft node
	lastHeight uint64                 //The latest height.
	cfg        *config.BftConfig      //bft config
	pn         p2pnode.Node           //p2p node
	bc         blockchain.Blockchains //blockchain
	pool       *txpool.TxPool         //txpool
}

//RequestManage struct
type RequestManage struct {
	bn *bftnode //bft node
}

//ReqNodeOption request
// type ReqNodeOption struct {
// 	GetLeader bool
// 	Addr      string //request address
// }

//ReqBlockrpc requests blocks from height 'LowH' to 'HeiH'.
type ReqBlockrpc struct {
	GetLeader    bool
	Addr         string //request address
	ReqMaxHeight bool   //request leader max block height
	ReqBlocks    bool   //request leader blocks from height 'LowH' to 'HeiH'
	LowH         uint64 //form LowH
	HeiH         uint64 //to HeiH
}

//ReSBlockrpc result info
type ReSBlockrpc struct {
	Data       []byte //blocks data
	LeaderAddr string
	MaxHieght  uint64 //leader max block height
}
