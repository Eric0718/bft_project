package bftnode

import (
	"encoding/json"
	"fmt"
	"kortho/logger"
	"net"
	"net/http"
	"net/rpc"

	"go.uber.org/zap"
)

/*
	// client, err := rpc.DialHTTP("tcp", leaderaddr)
	// if err != nil {
	// 	return nil, err
	// }

	// req := ReqBlockrpc{
	// 	ReqHeight: true,
	// }
	// res := ReSBlockrpc{Done: false}

	// err := client.Call("RequestManage.HandleGetLeaderMaxBlockHeight", &req, &res)
	// if err != nil {
	// 	logger.Error("Call HandleGetLeaderMaxBlockHeight error", zap.Error(err))
	// 	return 0, err
	// }RequestManage.HandleGetLeader
*/

//RunRPCServer Register a rpc for follows to request to recover blocks data.
func RunRPCServer(n *bftnode) error {
	rm := RequestManage{bn: n}
	err := rpc.Register(&rm)
	if err != nil {
		return fmt.Errorf("Register rpc error:%v", err)
	}

	rpc.HandleHTTP()
	lis, err := net.Listen("tcp", rm.bn.cfg.MRpcAddr)
	if err != nil {
		return fmt.Errorf("Listen error:%v", err)
	}

	logger.Info("Run bft Rpc Server......", zap.String("rpc addr", rm.bn.cfg.MRpcAddr))
	er := http.Serve(lis, nil)
	if err != nil {
		return er
	}
	return nil
}

//HandleAddPeer leader add node
func (rm *RequestManage) HandleAddPeer(req ReqBlockrpc, res *ReSBlockrpc) error {
	if len(req.Addr) > 0 {
		return rm.bn.Add(req.Addr)
	}
	return fmt.Errorf("HandleAddPeer error:req.addr = %v", req.Addr)
}

//HandleRemovePeer leader remove node
func (rm *RequestManage) HandleRemovePeer(req ReqBlockrpc, res *ReSBlockrpc) error {
	if len(req.Addr) > 0 {
		return rm.bn.Remove(req.Addr)
	}
	return fmt.Errorf("HandleRemovePeert error:req.addr = %v", req.Addr)
}

//HandleGetLeader Get Leader
func (rm *RequestManage) HandleGetLeader(req ReqBlockrpc, res *ReSBlockrpc) error {
	if req.GetLeader {
		res.LeaderAddr = rm.bn.Bn.GetLeader()
	}
	return nil
}

//HandleGetBlockSection Get Block Section
func (rm *RequestManage) HandleGetBlockSection(req ReqBlockrpc, res *ReSBlockrpc) error {
	blocks, err := rm.bn.bc.GetBlockSection(req.LowH, req.HeiH)
	if err != nil {
		return fmt.Errorf("Failed to request blocks from leader:%v", err)
	}
	d, err := json.Marshal(blocks)
	if err != nil {
		return fmt.Errorf("HandleGetBlockSection json Marshal failed:%v", err)
	}
	res.Data = d
	return nil
}

//HandleGetLeaderMaxBlockHeight gets leader max block height
func (rm *RequestManage) HandleGetLeaderMaxBlockHeight(req ReqBlockrpc, res *ReSBlockrpc) error {
	if req.ReqMaxHeight {
		h, err := rm.bn.bc.GetHeight()
		if err != nil {
			return fmt.Errorf("Request leader max block height error:%v", err)
		}
		res.MaxHieght = h
		return nil
	}
	return fmt.Errorf("Request leader max block height failed,req.ReqHeight = %v", req.ReqMaxHeight)
}

//HandleGetBlockByHeight get block by height
func (rm *RequestManage) HandleGetBlockByHeight(req ReqBlockrpc, res *ReSBlockrpc) error {
	if req.ReqBlocks {
		b, err := rm.bn.bc.GetBlockByHeight(req.LowH)
		if err != nil {
			return fmt.Errorf("Request get block by height[%v] error:%v", req.LowH, err)
		}
		d, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("Request get block by height[%v] json Marshal error:%v", req.LowH, err)
		}
		res.Data = d
		return nil
	}
	return fmt.Errorf("Request get block by height[%v] failed! option = %v", req.LowH, req.ReqBlocks)
}

//HandleGetMaxBlockHeight get block by height
func (rm *RequestManage) HandleGetMaxBlockHeight(req ReqBlockrpc, res *ReSBlockrpc) error {
	if req.ReqMaxHeight {
		h, err := rm.bn.bc.GetHeight()
		if err != nil {
			return fmt.Errorf("Request max block height error:%v", err)
		}
		res.MaxHieght = h
		return nil
	}
	return fmt.Errorf("Request max block height failed,req.ReqHeight = %v", req.ReqMaxHeight)
}
