package bftnode

import (
	"encoding/json"
	"fmt"
	"kortho/logger"
	"net"
	"net/http"
	"net/rpc"
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
	// }
*/

//RunRequestBlockRPC Register a rpc for follows to request to recover blocks data.
func (rm *RequestManage) RunRequestBlockRPC() error {

	err := rpc.Register(rm)
	if err != nil {
		return fmt.Errorf("Register rpc error:%v", err)
	}

	rpc.HandleHTTP()

	lis, err := net.Listen("tcp", rm.bn.cfg.RpcAddr)
	if err != nil {
		return fmt.Errorf("Listen error:%v", err)
	}

	logger.Info("Request Block Rpc Serve start......")
	er := http.Serve(lis, nil)
	if err != nil {
		return er
	}
	return nil
}

//HandleGetBlockSection Handle the request when the follows try to recover the backward blocks data by this rpc
func (rm *RequestManage) HandleGetBlockSection(req *ReqBlockrpc, res *ReSBlockrpc) error {
	if req.ReqBlocks {
		blocks, err := rm.bn.bc.GetBlockSection(req.LowH, req.HeiH)
		if err != nil {
			res.Done = false
			return fmt.Errorf("Failed to request blocks from leader")
		}
		res.Data, err = json.Marshal(blocks)
		if err != nil {
			res.Done = false
			res.Data = nil
			return fmt.Errorf("HandleGetBlockSection json Marshal failed:%v", err)
		}
		res.Done = true
		return nil
	}
	res.Done = false
	return fmt.Errorf("Failed to request blocks from leader!,req.ReqBlocks = %v", req.ReqBlocks)
}

//HandleGetLeaderMaxBlockHeight gets leader max block height
func (rm *RequestManage) HandleGetLeaderMaxBlockHeight(req *ReqBlockrpc, res *ReSBlockrpc) error {
	if req.ReqHeight {
		h, err := rm.bn.bc.GetHeight()
		if err != nil {
			res.Done = false
			return fmt.Errorf("Request leader max block height error:%v", err)
		}
		res.Done = true
		res.MaxHieght = h
		return nil
	}
	res.Done = false
	return fmt.Errorf("Request leader max block height failed,req.ReqHeight = %v", req.ReqHeight)
}
