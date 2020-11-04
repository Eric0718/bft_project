package monitor

import (
	pb "kortho/api/message"
	"kortho/blockchain"
)

type monitor struct {
	startBlockHeight uint64
	peer             string
	peers            []string
	grpcPort         string
	rpcPort          string
	raftPort         string
	bc               *blockchain.Blockchain
	accountAddr      string
	falseCount       uint
	maxBlockHeight   uint64

	peersLen int
	mBlocks  map[string]*pb.RespBlock
}

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
