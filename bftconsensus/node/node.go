//Package node actually deals with raft log.
package node

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"kortho/api"
	pb "kortho/api/message"
	"kortho/bftconsensus/protocol"
	"kortho/block"
	"kortho/blockchain"
	"kortho/logger"
	"kortho/transaction"
	"kortho/txpool"
	"kortho/types"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

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
	logger.Info("Into Apply ", zap.Uint64("e.Index", e.Index), zap.Uint64("e.term", e.Term))
	var leaderLastHeight uint64
	var CConn *grpc.ClientConn

	blockData := struct {
		Block *block.Block `json:"block"`
	}{}

	if err := json.Unmarshal(e.Data, &blockData); err != nil {
		logger.Error("Apply json.Unmarshal error", zap.Error(err))
		return err
	}
	b := blockData.Block

	//already commited.
	if b.Height < 1+n.currentHeight {
		logger.Info("height already commited,end apply", zap.Uint64("b.Height", b.Height), zap.Uint64("Current Height", n.currentHeight))
		return nil
	}

	leader := n.cp.GetLeader()
	if leader == "" {
		logger.Error("Error: the cluster no leader now,return apply")
		return nil
	}
	logger.Info("Leader Address:", zap.String("Current leader", leader), zap.String("Node Address", n.nodeAddr))

	if leader != n.nodeAddr {
		//only follows need to build connection with leader
		CConn = n.getRPCClient()
		if CConn == nil {
			logger.Error("getRPCClient failed!")
			return nil
		}
		defer CConn.Close()

		//only follows need to get the max Block Height From Leader
		breakC := time.After(2 * time.Second)
		for {
			select {
			case <-breakC:
				logger.Error("getMaxBlockHeightFromLeader timeout,return...")
				return nil
			case <-time.After(time.Millisecond):
				h, err := n.getMaxBlockHeightFromLeader(CConn)
				if err != nil {
					logger.Error("getMaxBlockHeightFromLeader failed,continue to get...", zap.Error(err))
					continue
				} else {
					leaderLastHeight = h
					break
				}
			}
			break
		}
		logger.Info("node status:", zap.Uint64("n.currentHeight", n.currentHeight), zap.Uint64("b.Height", b.Height), zap.Uint64("leaderLastHeight", leaderLastHeight))
		//commit without check to catch up with leader.
		if b.Height <= leaderLastHeight {
			if n.currentHeight+1 == b.Height {
				err := n.commitF(n.u, e.Data)
				if err != nil {
					logger.Error("commit block error", zap.Error(err))
					return err
				}
				n.currentHeight = b.Height
				logger.Info("End Apply without delive")
			} else { //recover backward blocks by rpc
				err := n.recoverBackwardBlocks(CConn, n.currentHeight+1, leaderLastHeight)
				if err != nil {
					logger.Error("recoverBackwardBlocks error", zap.Error(err))
					return err
				}
			}
			n.deleteCommitedBlockInMap(leaderLastHeight)
			return nil
		}
	}

	if b.Height > 1+n.currentHeight && !n.IsMiner() {
		err := n.recoverBackwardBlocks(CConn, n.currentHeight+1, leaderLastHeight)
		if err != nil {
			logger.Error("recoverBackwardBlocks error", zap.Error(err))
			return err
		}
		n.deleteCommitedBlockInMap(leaderLastHeight)
	}

	if b.Height == 1+n.currentHeight {
		//callback delive function
		n.deliveF(n.u, e.Data)

		breakC := time.After(time.Minute)  //return when timeout
		deliveC := time.After(time.Second) //delive per 5 seconds.
		for {
			select {
			case <-breakC:
				logger.Error("Apply error:timeout,return......")
				if n.IsMiner() {
					logger.Debug("Do LeaderShip Transfer To Follow.")
					err := n.LeaderShipTransferToF()
					if err != nil {
						logger.Error("LeaderShipTransferToF error", zap.Error(err))
					}
				}
				return nil
			case <-deliveC:
				logger.Info("delive again...")
				n.deliveF(n.u, e.Data)
			case <-time.After(time.Millisecond * 2):
				if n.handleBlockData(b.Height, b.Hash, e.Data) {
					n.deleteCommitedBlockInMap(b.Height)
					logger.Info("End Apply", zap.Uint64("current height", n.currentHeight))
					return nil
				}
			}
		}
	}
	return nil
}

//deal with a block data which needs to commit when more than 2/3 nodes checked true or over 1/3 are false.
func (n *node) handleBlockData(hei uint64, hs []byte, data []byte) bool {
	var trueCount, falseCount uint64
	n.pool.Mutex.RLock()
	defer n.pool.Mutex.RUnlock()
	for _, v := range n.pool.Idhc {
		if v.Height == hei {
			if bytes.Equal(v.Hash, hs) && v.Code {
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
		logger.Error("handleBlockData error: Over 1/3 nodes checked failed and abandoned to commit block", zap.Uint64("ok Count", trueCount), zap.Uint64("failed Count", falseCount))
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

func (n *node) getRPCClient() *grpc.ClientConn {
	breakC := time.After(time.Minute)
	for {
		select {
		case <-breakC:
			//restart when timeout
			logger.Error("getRPCClient error timeout,return......")
			return nil
		case <-time.After(time.Millisecond):
			addr := n.cp.GetLeader()
			ad := strings.Split(addr, ":")
			leaderaddr := ad[0] + n.rpcPort
			conn, err := grpc.Dial(leaderaddr, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				logger.Error("grpc Dial error", zap.Error(err))
				continue
			}
			return conn
		}
	}
}

//Get the latest height from leader.
func (n *node) getMaxBlockHeightFromLeader(CConn *grpc.ClientConn) (uint64, error) {

	cc := pb.NewGreeterClient(CConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res, err := cc.GetMaxBlockNumber(ctx, &pb.ReqMaxBlockNumber{})
	if err != nil {
		logger.Error("Call HandleGetLeaderMaxBlockHeight error", zap.Error(err))
		return 0, err
	}

	return res.MaxNumber, nil
}

//recover backward blocks data from height 'lo' to 'hi'.
func (n *node) recoverBackwardBlocks(CConn *grpc.ClientConn, lo uint64, hi uint64) error {
	logger.Info("Into recoverBackwardBlocks", zap.Uint64("low height", lo), zap.Uint64("high height", hi))

	cc := pb.NewGreeterClient(CConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if lo > hi {
		return fmt.Errorf("Wrong block interval：lo[%v] > hi[%v]", lo, hi)
	}

	for i := lo; i <= hi; i++ {
		logger.Info("Start to recover Backward Blocks", zap.Uint64("Height", i))
		res, err := cc.GetBlockByNum(ctx, &pb.ReqBlockByNumber{Height: i})
		if err != nil {
			logger.Error("recoverBackwardBlocks GetBlockByNum error", zap.Error(err))
			return err
		}
		if res == nil {
			continue
		}

		bc, err := n.blockConversion(res)
		if err != nil {
			return err
		}
		if bc.Height <= n.currentHeight {
			continue
		}

		if n.currentHeight+1 == bc.Height {
			bData := struct {
				Block *block.Block
			}{
				Block: bc,
			}
			data, err := json.Marshal(bData)
			if err != nil {
				logger.Error("json Marshal error", zap.Error(err))
				return err
			}
			err = n.commitF(n.u, data)
			if err != nil {
				logger.Error("commit block error", zap.Error(err))
				return err
			}
			n.currentHeight = bc.Height
		}

	}

	logger.Info("End recoverBackwardBlocks", zap.Uint64("current height", n.currentHeight))
	return nil
}

func (n *node) blockConversion(res *pb.RespBlock) (*block.Block, error) {
	var Tx []*transaction.Transaction

	if len(res.Txs) > 0 {
		for _, msTx := range res.Txs {
			if msTx != nil {
				t, err := api.MsgTxToTx(msTx)
				if err != nil {
					return nil, err
				}
				Tx = append(Tx, t)
			}
		}
	}

	PrHs, err := hex.DecodeString(res.PrevBlockHash)
	if err != nil {
		return nil, err
	}

	Hs, err := hex.DecodeString(res.Hash)
	if err != nil {
		return nil, err
	}

	Rt, err := hex.DecodeString(res.Root)
	if err != nil {
		return nil, err
	}

	ad, err := types.StringToAddress(res.Miner)
	if err != nil {
		return nil, err
	}

	b := &block.Block{
		Height:       res.Height,
		PrevHash:     PrHs,
		Hash:         Hs,
		Transactions: Tx,
		Root:         Rt,
		Version:      res.Version,
		Timestamp:    res.Timestamp,
		Miner:        *ad,
	}
	return b, nil
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
