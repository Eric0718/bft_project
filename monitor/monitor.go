package monitor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	pb "kortho/api/message"
	"kortho/block"
	"kortho/blockchain"
	"kortho/config"
	"kortho/logger"
	"net/rpc"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	//BlockRange recover blocks range
	BlockRange = 999
)

var nodesNum int
var isContinue bool = false

//Run start monitor
func Run(cfg *config.MonitorConfig, bc *blockchain.Blockchain) {

	m := &monitor{
		startBlockHeight: cfg.StartBlockHeight,
		peer:             cfg.MPeer,
		peers:            cfg.MPeers,
		grpcPort:         cfg.GrpcPort,
		rpcPort:          cfg.RpcPort,
		raftPort:         cfg.RaftPort,
		accountAddr:      cfg.AccountAddr,
		bc:               bc,
		falseCount:       0,
		maxBlockHeight:   0,
		peersLen:         len(cfg.MPeers),
		mBlocks:          make(map[string]*pb.RespBlock),
	}

	if m.startBlockHeight == 0 {
		mbn, err := m.getMaxBlockHeight()
		if err != nil {
			logger.Error("getMaxBlockHeight error: %s\n", zap.Error(err))
			os.Exit(1)
		}
		if mbn == 0 {
			mbn++
		}
		m.startBlockHeight = mbn
	}
	logger.Info("Run Monitoring...", zap.Uint64("startHeight", m.startBlockHeight))
	go m.Monitoring()
}

//monitor every block
func (m *monitor) Monitoring() {
	for {
		time.Sleep(time.Millisecond * 200)
		nodesNum = len(m.peers)
		if !isContinue {
			mbn, err := m.getMaxBlockHeight()
			if err != nil {
				logger.Error("getMaxBlockHeight error: %s\n", zap.Error(err))
				continue
			}
			m.maxBlockHeight = mbn
			//get self start block: startB;
			startB, err := m.getStartBlock()
			if err != nil {
				//logger.Error("getStartBlock error:", zap.Error(err))
				continue
			}
			//get others Blocks []lbs by startB.Height;
			err = m.getOtherNodesBlockByHeight(startB.Height)
			if err != nil {
				logger.Error("getOtherNodesBlockByHeight error:", zap.Error(err))
				continue
			}
			//for compare(startB.Root,lbs[n].Root) ==> falseCount;
			m.falseCount = m.compareBlocks(startB)

			logger.Info("monitor info===>>>", zap.Uint64("start height", m.startBlockHeight), zap.Uint64("max height", m.maxBlockHeight), zap.Uint("falseCount", m.falseCount), zap.Int("nodesNum", nodesNum))
			if len(m.mBlocks) != nodesNum {
				continue
			}
			if m.falseCount == 0 {
				//clean Blocks:
				m.cleanBlocks()
				isContinue = false
				m.startBlockHeight++ //updata startBlockHeight
				continue
			}
		}

		//self error: Do recover data
		if int(m.falseCount) <= nodesNum/3 {
			if m.maxBlockHeight >= m.startBlockHeight {
				if !isContinue {
					//current node needs to be stoped to commit block,to prepar for recovering.
					if err := m.stopBFTNode(); err != nil {
						logger.Error("stop BFT Node failed!", zap.Error(err))
						continue
					}
				}

				//***********remove fork block chain:***********
				//deleteBlock,handleBalance
				err := m.bc.DeleteBlock(m.startBlockHeight)
				if err != nil {
					logger.Error("DeleteBlock error!", zap.Error(err))
					isContinue = true
					continue
				}

				//recover the main chain from leader
				err = m.recoverBlocks()
				if err != nil {
					logger.Error("recoverBlocks error!", zap.Error(err))
					isContinue = true
					continue
				}
				//restart raft
				go func() {
					for {
						if err := m.startBFTNode(); err != nil {
							logger.Error("start BFT Node failed!", zap.Error(err))
							isContinue = true
							continue
						}
						break
					}
					return
				}()

				//refresh these parameters.
				m.cleanBlocks()
				m.falseCount = 0
				isContinue = false
			}
		}
	}
}

//get max block height
func (m *monitor) getMaxBlockHeight() (uint64, error) {
	return m.bc.GetMaxBlockHeight()
}

//Get a block to compare with other nodes.
func (m *monitor) getStartBlock() (*block.Block, error) {
	return m.bc.GetBlockByHeight(m.startBlockHeight)
}

//get a block by height
func (m *monitor) getBlockByHeight(hi uint64, addr string) (*pb.RespBlock, error) {
	conn, err := grpc.Dial(getAddress(addr, m.grpcPort), grpc.WithInsecure()) //, grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("grpc Dial error:%v", err)
	}
	defer conn.Close()
	cc := pb.NewGreeterClient(conn)
	res, err := cc.GetBlockByNum(context.Background(), &pb.ReqBlockByNumber{Height: hi})
	if err != nil {
		return nil, fmt.Errorf("Call GetMaxBlockNumber error:%v", err)
	}
	return res, nil
}

//get Other Nodes Block By the same Height
func (m *monitor) getOtherNodesBlockByHeight(hi uint64) error {
	if len(m.peers) <= 0 {
		return fmt.Errorf("the length of peers <= 0")
	}
	for i := 0; i < m.peersLen; i++ {
		if len(m.peers[i]) <= 0 {
			continue
		} else {
			resb, err := m.getBlockByHeight(hi, m.peers[i])
			if err != nil {
				nodesNum-- //Prevent dead nodes from being counted
				logger.Error("Call getBlockByHeight error:", zap.String("node address", m.peers[i]), zap.Uint64("height", hi), zap.Int("nodesNum", nodesNum), zap.Error(err))
				continue
			}
			m.mBlocks[m.peers[i]] = resb
		}
	}
	if len(m.mBlocks) == 0 {
		return fmt.Errorf("the length of map mBlocks <= 0")
	}
	return nil
}

//compare block with other nodes.
func (m *monitor) compareBlocks(b *block.Block) uint {
	var falseCount uint = 0
	//for i := 0; i < m.peersLen; i++ {
	for k, v := range m.mBlocks {
		if b.Height == v.Height {
			if hex.EncodeToString(b.Root) != v.Root {
				logger.Error("Wrong Blocks, Root hash not equal:", zap.Uint64("Height", b.Height), zap.String("node address", m.peer), zap.String("hash", hex.EncodeToString(b.Hash)),
					zap.String("another node address", k), zap.String("other hash", v.Hash))
				falseCount++
			}
		}
	}
	return falseCount
}

func (m *monitor) cleanBlocks() {
	for i := 0; i < m.peersLen; i++ {
		delete(m.mBlocks, m.peers[i])
	}
}

//GetLeader get leader address
func GetLeader(addr string) (string, error) {
	logger.Info("Into GetLeader", zap.String("addr:", addr))
	client, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer client.Close()

	req := ReqBlockrpc{
		GetLeader: true,
	}
	res := ReSBlockrpc{}

	err = client.Call("RequestManage.HandleGetLeader", req, &res)
	if err != nil {
		return "", fmt.Errorf("Call HandleGetLeader error:%v", err)
	}
	return res.LeaderAddr, nil
}

func (m *monitor) getLeaderMaxBlockHeight() (uint64, error) {
	addr, err := GetLeader(m.peer + m.rpcPort)
	if err != nil {
		logger.Error("GetLeader error:", zap.Error(err))
		return 0, err
	}

	client, err := rpc.DialHTTP("tcp", getAddress(addr, m.rpcPort))
	if err != nil {
		return 0, err
	}
	defer client.Close()

	req := ReqBlockrpc{
		ReqMaxHeight: true,
	}
	res := ReSBlockrpc{}

	err = client.Call("RequestManage.HandleGetLeaderMaxBlockHeight", req, &res)
	if err != nil {
		return 0, fmt.Errorf("Call HandleGetLeaderMaxBlockHeight error:%v", err)
	}

	return res.MaxHieght, nil
}

//stop node when find a different block with other nodes.
func (m *monitor) stopBFTNode() error {
	logger.Info("Into stopBFTNode")

	addr, err := GetLeader(m.peer + m.rpcPort)
	if err != nil {
		logger.Error("GetLeader error:", zap.Error(err))
		return err
	}

	client, err := rpc.DialHTTP("tcp", getAddress(addr, m.rpcPort))
	if err != nil {
		return err
	}
	defer client.Close()

	req := ReqBlockrpc{
		Addr: m.peer + m.raftPort,
	}
	res := ReSBlockrpc{}
	logger.Info("stopBFTNode", zap.String("leader", addr), zap.String("rm addr", req.Addr))
	err = client.Call("RequestManage.HandleRemovePeer", req, &res)
	if err != nil {
		return fmt.Errorf("Call HandleRemovePeer error:%v", err)
	}
	logger.Info("stop Node Ok!")
	return nil
}

//start node when finished to recover blocks
func (m *monitor) startBFTNode() error {
	logger.Info("Into startBFTNode")
	var laddr string
	for _, adr := range m.peers {

		add, err := GetLeader(getAddress(adr, m.rpcPort))
		if err != nil {
			logger.Error("GetLeader error:", zap.Error(err))
			continue
		}
		laddr = add
		break
	}
	if len(laddr) <= 0 {
		return errors.New("startBFTNode GetLeader nil")
	}

	client, err := rpc.DialHTTP("tcp", getAddress(laddr, m.rpcPort)) //addr)
	if err != nil {
		return fmt.Errorf("startBFTNode rpc DialHTTP error:%v", err)
	}
	defer client.Close()

	time.Sleep(time.Millisecond * 5)
	req := ReqBlockrpc{
		Addr: m.peer + m.raftPort,
	}
	res := ReSBlockrpc{}

	err = client.Call("RequestManage.HandleAddPeer", req, &res)
	if err != nil {
		return fmt.Errorf("Call HandleAddPeer error:%v", err)
	}

	logger.Info("start Node Ok!")
	return nil
}

//recover fork chain from leader
func (m *monitor) recoverBlocks() error {
	logger.Info("recoverBlocks", zap.Uint64("from height", m.startBlockHeight), zap.Uint64("to height", m.maxBlockHeight))
	var laddr string
	for _, adr := range m.peers {
		add, err := GetLeader(getAddress(adr, m.rpcPort))
		if err != nil {
			logger.Error("GetLeader error:", zap.Error(err))
			continue
		}
		laddr = add
		break
	}
	if len(laddr) <= 0 {
		return fmt.Errorf("recoverBlocks GetLeader nil")
	}

	client, err := rpc.DialHTTP("tcp", getAddress(laddr, m.rpcPort)) //laddress)
	if err != nil {
		return err
	}
	defer client.Close()

	lo := m.startBlockHeight
	for hi := m.startBlockHeight + BlockRange; hi <= m.maxBlockHeight; hi = lo + BlockRange {
		data, err := getBlocks(lo, hi, client)
		if err != nil {
			return err
		}
		err = m.commitBlocks(data)
		if err != nil {
			return err
		}
		lo = hi + 1
	}

	if m.maxBlockHeight >= lo {
		data, err := getBlocks(lo, m.maxBlockHeight, client)
		if err != nil {
			return err
		}
		err = m.commitBlocks(data)
		if err != nil {
			return err
		}
	}
	logger.Info("End recoverBlocks")
	return nil
}

func getBlocks(lowH uint64, hiH uint64, conn *rpc.Client) ([]byte, error) {
	logger.Info("Into getBlocks")
	req := ReqBlockrpc{
		LowH: lowH,
		HeiH: hiH,
	}
	res := ReSBlockrpc{}

	err := conn.Call("RequestManage.HandleGetBlockSection", req, &res)
	if err != nil {
		return nil, fmt.Errorf("Call HandleGetBlockSection error:%v", err)
	}
	logger.Info("Finished getBlocks")
	return res.Data, nil
}

func (m *monitor) commitBlocks(data []byte) error {
	var blks []*block.Block
	err := json.Unmarshal(data, &blks)
	if err != nil {
		return fmt.Errorf("Call json.Unmarshal error:%v", err)
	}

	if len(blks) <= 0 {
		return fmt.Errorf("len(blks) <= 0")
	}

	for _, b := range blks {
		if b == nil {
			continue
		}

		err := m.bc.RecoverBlock(b, []byte(m.accountAddr))
		if err != nil {
			logger.Error("commit block error", zap.Error(err))
			return err
		}
		//m.startBlockHeight = b.Height
	}
	return nil
}

func getAddress(addr string, port string) string {
	ad := strings.Split(addr, ":")
	return ad[0] + port
}
