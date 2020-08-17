//Package node actually deals with raft log.
package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kortho/bftconsensus/protocol"
	"kortho/block"
	"kortho/blockchain"
	"kortho/logger"
	"kortho/txpool"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

const recoverNum = 1000 //total blocks number when request backward data from leader

//New node
func New(cfg *Config, u interface{}, cf CommitFunc, df DeliveFunc, bc blockchain.Blockchains, pool *txpool.TxPool) (Node, error) {
	var n node
	pC := &protocol.Config{
		Join:              cfg.Join,
		Address:           cfg.Address,
		SnapshotInterval:  cfg.SnapshotInterval,
		SnapshotThreshold: cfg.SnapshotThreshold,
		LogDir:            cfg.LogDir,
		SnapDir:           cfg.SnapDir,
		LogsDir:           cfg.LogsDir,
		StableDir:         cfg.StableDir,
	}
	cp, err := protocol.New(pC, &n)
	if err != nil {
		logger.Error("protocol New error:", zap.Error(err))
		return nil, err
	}
	{
		logger.Info("init...")
	}
	n.u = u
	n.cp = cp
	n.commitF = cf
	n.deliveF = df
	n.pool = pool
	n.boot = cfg.Join
	n.nodeN = cfg.NodeNum
	n.bc = bc
	n.rpcPort = cfg.RPCPort
	n.recPort = cfg.RecPort
	n.nodeAddr = cfg.Address
	chi, err := n.bc.GetHeight()
	if err != nil {
		return nil, err
	}
	n.currentHeight = chi

	return &n, nil
}

//spread a block data to other nodes by p2p
func (n *node) Prepare(data []byte) error {
	return n.cp.Prepare(data)
}

//deal with raft log.
func (n *node) Apply(e *raft.Log) interface{} {
	logger.Info("Into Apply ", zap.String("Current leader:", n.cp.GetLeader()), zap.String("Self Address", n.nodeAddr), zap.Uint64("e.Index", e.Index), zap.Uint64("e.term", e.Term))

	client, err := n.getRPCClient()
	if err != nil {
		logger.Error("get Rpc Connection error", zap.Error(err))
		return err
	}
	defer client.Close()

	blockData := struct {
		Block *block.Block `json:"block"`
	}{}

	if err = json.Unmarshal(e.Data, &blockData); err != nil {
		return err
	}
	b := blockData.Block

	//already commited.
	if b.Height <= n.currentHeight {
		logger.Info("height already commited,end apply", zap.Uint64("b.Height", b.Height), zap.Uint64("Current Height", n.currentHeight))
		return nil
	}

	var leaderLastHeight uint64
	if n.cp.GetLeader() != n.nodeAddr {
		breakC := time.After(3 * time.Second)
		for {
			select {
			case <-breakC:
				logger.Error("getLastHeightFromLeader failed,timeout...")
				return errors.New("getLastHeightFromLeader failed: request timeout")
			case <-time.After(time.Millisecond * 10):
				h, err := n.getLastHeightFromLeader(client)
				if err != nil {
					logger.Error("getLastHeightFromLeader failed,continue to get...")
					continue
				} else {
					leaderLastHeight = h
					break
				}
			}
			break
		}

		logger.Info("node status:", zap.Uint64("n.currentHeight", n.currentHeight), zap.Uint64("b.Height", b.Height), zap.Uint64("leaderLastHeight", leaderLastHeight))

		//commit without check when restart.
		if b.Height <= leaderLastHeight {
			if n.currentHeight+1 == b.Height {
				err := n.commitF(n.u, e.Data)
				if err != nil {
					logger.Error("commit block error", zap.Error(err))
					return err
				}
				n.deleteCommitedBlockInMap(leaderLastHeight)
				n.currentHeight = b.Height
			} else { //recover backward blocks by rpc
				i := n.currentHeight + 1
				for j := (i + recoverNum); j <= leaderLastHeight; j = i + recoverNum {
					err := n.recoverBackwardBlocks(client, i, j)
					if err != nil {
						logger.Error("recoverBackwardBlocks error", zap.Error(err))
						return err
					}
					i = n.currentHeight + 1
				}
				if n.currentHeight+1 < leaderLastHeight {
					err := n.recoverBackwardBlocks(client, i, leaderLastHeight)
					if err != nil {
						logger.Error("recoverBackwardBlocks error", zap.Error(err))
						return err
					}
				}
			}
			logger.Info("End Apply without delive")
			return nil
		}
	}

	if b.Height == 1+n.currentHeight {
		//callback delive function
		n.deliveF(n.u, e.Data)

		breakC := time.After(time.Minute * 5)  //exit when timeout
		deliveC := time.After(time.Second * 5) //delive per 5 seconds.
		for {
			select {
			case <-breakC:
				//restart when timeout
				logger.Error("Apply error:timeout,restart......")
				os.Exit(1)
			case <-deliveC:
				logger.Info("delive again...")
				n.deliveF(n.u, e.Data)
			case <-time.After(time.Millisecond * 10):
				if n.handleBlockData(b.Height, e.Data) {
					n.deleteCommitedBlockInMap(b.Height)
					logger.Info("End Apply", zap.Uint64("current height", n.currentHeight))
					return nil
				}
			}
		}
	}
	return errors.New("Wrong raft log block data")
}

//deal with a block data which needs to commit when more than 2/3 nodes checked true or over 1/3 are false.
func (n *node) handleBlockData(hei uint64, data []byte) bool {
	var trueCount, falseCount uint64
	n.pool.Mutex.RLock()
	defer n.pool.Mutex.RUnlock()
	for _, v := range n.pool.Idhc {
		if v.Height == hei {
			if v.Code {
				trueCount++
			} else {
				falseCount++
			}
		}
	}

	//more than 2/3 nodes checked block ok to commit.
	if trueCount >= n.nodeN*2/3 {
		if n.currentHeight+1 == hei {
			err := n.commitF(n.u, data)
			if err != nil {
				logger.Error("commit block error", zap.Error(err))
				return false
			}
			n.currentHeight = hei
			logger.Info("Over 2/3 nodes checked ok and commited.", zap.Uint64("ok Count", trueCount), zap.Uint64("failed Count", falseCount))
			return true
		}
	}

	//over 1/3 nodes checked false,the block can not to be commited.
	if falseCount > n.nodeN/3 {
		logger.Error("handleBlockData error: Over 1/3 nodes checked failed and abandoned to commit block", zap.Uint64("ok Count", trueCount), zap.Uint64("failed Count", falseCount), zap.Uint64("total nodes / 3", n.nodeN/3))
		return false
	}
	return false
}

//delete already commited blocks from map.
func (n *node) deleteCommitedBlockInMap(hei uint64) {
	n.pool.Mutex.Lock()
	defer n.pool.Mutex.Unlock()
	if len(n.pool.Idhc) == 0 {
		return
	}

	for k, v := range n.pool.Idhc {
		if v.Height <= hei {
			delete(n.pool.Idhc, k)
		}
	}
	logger.Info("finished delete commited height in map.", zap.Uint64("height", hei), zap.Int("map length", len(n.pool.Idhc)))
}

func (n *node) getRPCClient() (*rpc.Client, error) {
	addr := n.cp.GetLeader()
	ad := strings.Split(addr, ":")
	leaderaddr := ad[0] + n.rpcPort
	//logger.Info("start to get rpc connection...", zap.String("leader address", leaderaddr))
	client, err := rpc.DialHTTP("tcp", leaderaddr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

//Get the latest height from leader.
func (n *node) getLastHeightFromLeader(client *rpc.Client) (uint64, error) {

	req := ReqBlockrpc{
		ReqHeight: true,
	}
	res := ReSBlockrpc{Done: false}

	err := client.Call("RequestManage.HandleGetLeaderMaxBlockHeight", &req, &res)
	if err != nil {
		logger.Error("Call HandleGetLeaderMaxBlockHeight error", zap.Error(err))
		return 0, err
	}
	if !res.Done {
		return 0, fmt.Errorf("request leader max block height failed.res.Done = %v", res.Done)
	}

	return res.MaxHieght, nil
}

//recover backward blocks data from height 'lo' to 'hi'.
func (n *node) recoverBackwardBlocks(client *rpc.Client, lo uint64, hi uint64) error {
	logger.Info("Into recoverBackwardBlocks", zap.Uint64("low height", lo), zap.Uint64("high height", hi))
	n.deleteCommitedBlockInMap(hi)
	req := ReqBlockrpc{
		ReqBlocks: true,
		LowH:      lo,
		HeiH:      hi,
	}
	res := ReSBlockrpc{Done: false}

	//request backward data from leader by rpc.
	err := client.Call("RequestManage.HandleGetBlockSection", &req, &res)
	if err != nil {
		logger.Error("Call HandleGetBlockSection error", zap.Error(err))
		return err
	}

	var blocks []block.Block
	if res.Done && len(res.Data) > 0 {
		err := json.Unmarshal(res.Data, &blocks)
		if err != nil {
			logger.Error("json Unmarshal error", zap.Error(err))
			return err
		}
		//commit request backward data
		if blocks != nil && len(blocks) > 0 {
			for _, bs := range blocks {
				if bs.Height <= n.currentHeight { //discard already commited blocks.
					continue
				}
				if n.currentHeight+1 == bs.Height {
					data, err := json.Marshal(bs)
					if err != nil {
						logger.Error("json Marshal error", zap.Error(err))
						return err
					}
					err = n.commitF(n.u, data)
					if err != nil {
						logger.Error("commit block error", zap.Error(err))
						return err
					}
					n.currentHeight = bs.Height
				}
			}
			logger.Info("End recoverBackwardBlocks", zap.Uint64("current height", n.currentHeight))
			return nil
		}
	}
	return fmt.Errorf("recoverBackwardBlocks failed: res.Done= %v,request Data lenght= %v,blocks lenght= %v", res.Done, len(res.Data), len(blocks))
}

//generate a snapshot struct
func (n *node) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{}, nil
}

//recover data
func (n *node) Restore(_ io.ReadCloser) error {
	return nil
}

//save the FSM snapshot out to the given sink
func (s *snapshot) Persist(_ raft.SnapshotSink) error {
	return nil
}

func (s *snapshot) Release() {}
