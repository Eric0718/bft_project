//Package api 提供了查询、发送等功能的RPC和HTTP接口
package api

import (
	"kortho/blockchain"
	"kortho/config"
	"kortho/p2p/node"
	"kortho/txpool"

	"github.com/buaazp/fasthttprouter"
)

// Start 启动api服务，包含rpc和http两种服务
func Start(cfg *config.APIConfigInfo, bc *blockchain.Blockchain, tp *txpool.TxPool, n node.Node) {
	greeter := newGreeter(cfg.RPCConfig, bc, tp, n)
	go greeter.RunRPC()

	blockChian = bc
	server := &Server{cfg.WEBConfig.Address, fasthttprouter.Router{}}
	server.Run()
}
