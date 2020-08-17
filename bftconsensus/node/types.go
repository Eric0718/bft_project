//Package node actually deals with raft log.
package node

import (
	"kortho/bftconsensus/protocol"
	"kortho/blockchain"
	"kortho/txpool"
	"sync"
)

//Config for node
type Config struct {
	Join              bool   //use for distinguishing leader does some actions.
	Address           string //node address
	SnapshotThreshold uint64 //Snapshot threshold
	SnapshotInterval  uint64 //Snapshot interval
	LogDir            string //Log location
	SnapDir           string //Snap location
	LogsDir           string //raft log location
	StableDir         string //raft stable location
	NodeNum           uint64 //node number
	RPCPort           string //request max block height rpc
	RecPort           string //request backward blocks data rpc
}

//Node interface
type Node interface {
	IsMiner() bool        //leader or not
	DelPeer(string) error //delete a node
	AddPeer(string) error //add a node
	Prepare([]byte) error //Prepare a block data
}

type node struct {
	u             interface{}
	commitF       CommitFunc             //callback function for commit block data
	deliveF       DeliveFunc             //callback function for delive block data
	cp            protocol.Consensus     //bft  Consensus
	bc            blockchain.Blockchains //blockchain
	pool          *txpool.TxPool         //TxPool
	nodeN         uint64                 //total nodes number
	boot          bool                   //use for leader starts a cluster
	currentHeight uint64                 //current height
	rpcPort       string                 //port for get max block height from leader
	recPort       string                 //port for recover blocks data
	mu            sync.RWMutex           //
	nodeAddr      string                 //node address
}

//true leader,false follow
func (n *node) IsMiner() bool {
	return n.cp.IsMiner()
}

//delete a node into cluster
func (n *node) DelPeer(id string) error {
	return n.cp.DelPeer(id)
}

//add a node into cluster
func (n *node) AddPeer(id string) error {
	return n.cp.AddPeer(id, id)
}

//CommitFunc commits the blocks
type CommitFunc (func(interface{}, []byte) error)

//DeliveFunc delive the blocks
type DeliveFunc (func(interface{}, []byte))

type snapshot struct {
}

//ReSBlockrpc result info
type ReSBlockrpc struct {
	Done      bool   //'true' requests successfully,'false' failed.
	Data      []byte //blocks data
	MaxHieght uint64 //leader max block height
}

//ReqBlockrpc requests blocks from height 'LowH' to 'HeiH'.
type ReqBlockrpc struct {
	ReqHeight bool   //request leader max block height
	ReqBlocks bool   //request leader blocks from height 'LowH' to 'HeiH'
	LowH      uint64 //form LowH
	HeiH      uint64 //to HeiH
}
