package bftnode

import (
	"kortho/blockchain"
	"kortho/config"
	"kortho/logger"
	"kortho/p2p/node"
	"kortho/txpool"
	"os"

	"go.uber.org/zap"
)

//RunbftNode Start a bft node.
func RunbftNode(cfg *config.BftConfig, bc blockchain.Blockchains, pn node.Node, pool *txpool.TxPool) {
	logger.Info("Start to run bft node...")
	n, err := NewBftNode(cfg, bc, pn, pool)
	if err != nil {
		logger.Error("NewBftNode error!", zap.Error(err))
		os.Exit(1)
	}

	go n.Run()
}
