//Package bftnode implements to package,check,delive and commit a block.
package bftnode

import (
	"encoding/json"
	"errors"
	"fmt"
	"kortho/bftconsensus/node"
	"kortho/block"
	"kortho/blockchain"
	"kortho/config"
	"kortho/logger"
	p2pnode "kortho/p2p/node"
	"kortho/txpool"
	addrtypes "kortho/types"
	"os"
	"time"

	"go.uber.org/zap"
)

//NewBftNode new a bft node.
func NewBftNode(cfg *config.BftConfig, bc blockchain.Blockchains, pn p2pnode.Node, pool *txpool.TxPool) (Node, error) {
	var bn bftnode
	nC := &node.Config{
		Join:              cfg.Join,
		Address:           cfg.NodeAddr,
		SnapshotThreshold: cfg.SnapshotCount,
		SnapshotInterval:  cfg.SnapshotInterval,
		LogDir:            cfg.LogDir,
		SnapDir:           cfg.SnapDir,
		LogsDir:           cfg.LogsDir,
		StableDir:         cfg.StableDir,
		NodeNum:           cfg.NodeNum,
		RPCPort:           cfg.RpcPort,
		MRpcAddr:          cfg.MRpcAddr,
	}
	n, err := node.New(nC, &bn, commit, delive, bc, pool)
	if err != nil {
		return nil, err
	}

	bn.Bn = n
	bn.pn = pn
	bn.cfg = cfg
	bn.bc = bc
	bn.pool = pool

	lh, err := bn.bc.GetHeight()
	if err != nil {
		return nil, err
	}

	bn.lastHeight = lh

	return &bn, nil
}

//run a bft node.This function packages a new block per second.
func (n *bftnode) Run() {
	logger.Info("Run bftnode...")
	//Only leader can do this when first time to start a cluster.
	if n.cfg.Join {
		go func() {
			for {
				if leader := n.Bn.GetLeader(); leader == n.cfg.NodeAddr {
					for _, peer := range n.cfg.Peers {
						logger.Info("leader add node", zap.String("node addr", peer))
						if err := n.Add(peer); err != nil {
							logger.Error("Add bft nodes failed! Please check 'peers' in config file,then restart.", zap.String("peer", peer))
							os.Exit(1)
						}
					}
					break
				}
				time.Sleep(time.Millisecond * 10)
			}
		}()
	}

	go func() {
		err := RunRPCServer(n)
		if err != nil {
			os.Exit(1)
		}
	}()

	time.Sleep(time.Second * 2)

	for {
		time.Sleep(time.Second)
		if leader := n.Bn.GetLeader(); leader == n.cfg.NodeAddr {
			txs := n.pool.Pending(n.bc)
			minerAddr, _ := addrtypes.BytesToAddress([]byte(n.cfg.CountAddr))
			dsAddr, _ := addrtypes.BytesToAddress([]byte(n.cfg.Ds))
			cmAddr, _ := addrtypes.BytesToAddress([]byte(n.cfg.Cm))
			qtjAddr, _ := addrtypes.BytesToAddress([]byte(n.cfg.QTJ))
			b, err := n.bc.NewBlock(txs, *minerAddr, *dsAddr, *cmAddr, *qtjAddr)
			if err != nil {
				logger.Error("Leader: PackBlock failed,do it again.", zap.Error(err))
				continue
			}
			if b.Height != 1+n.lastHeight {
				logger.Error("PackBlock error: b.Height != 1 + n.lastHeight.", zap.Uint64("b.Height", b.Height), zap.Uint64("lastHeight", n.lastHeight))
				continue
			}
			b.Miner = *minerAddr
			resultHash, err := n.bc.CalculationResults(b)
			if err != nil {
				logger.Error("failed to calculation results", zap.Error(err))
				continue
			}

			fmt.Println("pack info:", "Height", b.Height, "res hash", resultHash)

			blockData := DataStu{
				Block:      b,
				ResultHash: resultHash,
			}

			pb, err := json.Marshal(blockData)
			if err != nil {
				logger.Error("Marshal packaged block error!", zap.Error(err))
				continue
			}
			logger.Info("Prepare new block :", zap.Uint64("height", b.Height))
			if err := n.Bn.Prepare(pb); err != nil {
				logger.Error("error: Leader Prepare new block failed!", zap.Uint64("height", b.Height), zap.Error(err))
			}
		}

	}
}

//DataStu add ResultHash
type DataStu struct {
	//TODO：处理数据为nil时，引起panic的bug
	Block      *block.Block `json:"block"`
	ResultHash []byte       `json:"resulthash"`
}

//add other nodes into cluster.
func (n *bftnode) Add(addr string) error {
	return n.Bn.AddPeer(addr)
}

func (n *bftnode) Remove(addr string) error {
	return n.Bn.DelPeer(addr)
}

//This is a callback function to delive a correct bft log needs to be processed.
func delive(u interface{}, data []byte) error {
	logger.Info("delive start")
	//cb := txpool.CheckBlock{}
	//hs, he, err := checkBlockData(u, data)
	// if err == nil {
	// 	cb.Code = true
	// } else {
	// 	logger.Error("delive checkBlockData error!", zap.Error(err), zap.Bool("Code", false))
	// 	cb.Code = false
	// 	os.Exit(1)
	// }

	// cb.Nodeid = u.(*bftnode).cfg.NodeAddr
	// cb.Hash = hs
	// cb.Height = he
	// data, er := json.Marshal(cb)
	// if er != nil {
	// 	logger.Error("delive json.Marshal error", zap.Error(er))
	// 	return
	// }

	// erro := u.(*bftnode).pool.SetCheckData(data)
	// if erro != nil {
	// 	logger.Error("node SetCheckData error.\n", zap.Error(erro))
	// 	return
	// }

	// data = append([]byte{'c'}, data...) //adding 'c' to make a distinction between block data and tx data.
	// u.(*bftnode).pn.Broadcast(data)

	_, _, err := checkBlockData(u, data)
	if err != nil {
		return err
	}
	logger.Info("delive end")
	return nil
}

//Check the block data before commit.
func checkBlockData(u interface{}, data []byte) ([]byte, uint64, error) {
	bn := u.(*bftnode)

	var blockData DataStu

	if err := json.Unmarshal(data, &blockData); err != nil {
		logger.Error("checkBlockData Unmarshal error", zap.Error(err))
		return nil, 0, err
	}
	b := blockData.Block
	if b == nil {
		logger.Error("block is nil")
		return nil, 0, errors.New("block data is nil")
	}

	//not an expect block data,so return error here.
	if b.Height != 1+bn.lastHeight {
		logger.Error("checkBlockData block error: b.Height != 1 + bn.lastHeight.", zap.Uint64("b.height", b.Height), zap.Uint64("last height", bn.lastHeight))
		return b.Hash, b.Height, errors.New("checkBlockData block error: b.Height != 1+bn.lastHeight")
	}

	p := bn.pool
	p.Filter(*b)

	//logger.Info("checkBlockData info", zap.Uint64("Height", b.Height), zap.ByteString("res hash", blockData.ResultHash))
	fmt.Println("checkBlockData info:", "Height", b.Height, "res hash", blockData.ResultHash)

	if !bn.checkBlock(b, blockData.ResultHash) {
		logger.Error("Follow checkBlock error!", zap.Uint64("hegiht:", b.Height))
		return b.Hash, b.Height, errors.New("CheckBlock ERROR")
	}

	if !txpool.VerifyBlock(*b, bn.bc) {
		logger.Error("Follow verifyBlcok error!", zap.Uint64("hegiht:", b.Height))
		return b.Hash, b.Height, errors.New("VerifyBlcok ERROR")
	}

	return b.Hash, b.Height, nil
}

//commit the correct block data.
func commit(u interface{}, data []byte) error {
	var blockData DataStu
	err := json.Unmarshal(data, &blockData)
	if err != nil {
		logger.Error("commit Unmarshal error", zap.Error(err))
		return fmt.Errorf("Commit block failed:%v", err)
	}
	b := blockData.Block
	if b == nil {
		logger.Error("failed to unmarsal block")
		return errors.New("Commit block failed")
	}

	err = u.(*bftnode).bc.AddBlock(b, []byte(u.(*bftnode).cfg.CountAddr))
	if err != nil {
		logger.Error("Fatal error: commit block failed", zap.Uint64("height", b.Height), zap.Error(err))
		return fmt.Errorf("Commit block failed:%v", err)
	}
	//update last blockHeight
	u.(*bftnode).lastHeight = b.Height
	logger.Info("Finished commit block", zap.Uint64("height", u.(*bftnode).lastHeight), zap.Int("data lenght", len(data)), zap.Int("tx lenght", len(b.Transactions)))
	return nil
}

//check block data.
func (n *bftnode) checkBlock(b *block.Block, resultHash []byte) bool {
	if !n.bc.CheckResults(b, resultHash, []byte(n.cfg.Ds), []byte(n.cfg.Cm), []byte(n.cfg.QTJ)) {
		return false
	}
	return true
}
