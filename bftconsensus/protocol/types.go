//This package implements to new a raft node and build cluster connections between nodes.
package protocol

import (
	"time"

	"github.com/hashicorp/raft"
)

type Consensus interface {
	//true is leader,false not.
	IsMiner() bool
	//Prepare block data and apply
	Prepare([]byte) error
	//delete a node
	DelPeer(string) error
	//add a node
	AddPeer(string, string) error
	//get leader address
	GetLeader() string
	LeaderShipTransferToF() error
	GetStats() map[string]string
}

type Config struct {
	Join              bool   //use for distinguishing leader does some actions.
	Address           string //node address
	SnapshotThreshold uint64 //Snapshot threshold
	SnapshotInterval  uint64 //Snapshot interval
	LogDir            string //Log location
	SnapDir           string //Snap location
	LogsDir           string //raft log location
	StableDir         string //raft stable location
}

type node struct {
	*raft.Raft
	fsm raft.FSM
}

func (a *node) IsMiner() bool {
	if err := a.VerifyLeader(); err.Error() != nil {
		return false
	}
	return true
}

func (a *node) Prepare(log []byte) error {
	if err := a.Apply(log, 10*time.Second); err.Error() != nil {
		return err.Error()
	}
	return nil
}

func (a *node) DelPeer(id string) error {
	if err := a.RemoveServer(raft.ServerID(id), 0, 0); err.Error() != nil {
		return err.Error()
	}
	return nil
}

func (a *node) AddPeer(id string, addr string) error {
	if err := a.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0); err.Error() != nil {
		return err.Error()
	}
	return nil
}

func (a *node) GetLeader() string {
	return string(a.Leader())
}

func (a *node) LeaderShipTransferToF() error {
	if err := a.LeadershipTransfer(); err.Error() != nil {
		return err.Error()
	}
	return nil
}

func (a *node) GetStats() map[string]string {
	return a.Stats()
}
