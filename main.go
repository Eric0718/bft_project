package main

import (
	"fmt"
	"os"

	"kortho/api"
	"kortho/bftconsensus/bftnode"
	"kortho/blockchain"
	"kortho/config"
	"kortho/logger"
	"kortho/monitor"
	"kortho/p2p/node"
	"kortho/transaction"
	"kortho/txpool"
	_ "net/http/pprof"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("load config failed:", err)
		os.Exit(-1)
	}

	transaction.InitAdmin(cfg.APIConfig.RPCConfig.AdminAddr)

	if err = logger.InitLogger(cfg.LogConfig); err != nil {
		fmt.Println("logger.InitLogger failed:", err)
		os.Exit(-1)
	}

	tp, err := txpool.New(cfg.BFTConfig.QTJ)
	if err != nil {
		logger.Error("Failed to new txpool", zap.Error(err))
		os.Exit(-1)
	}

	bc := blockchain.New()

	nB, err := node.New(cfg.P2PConfigList[0], tp, bc) //use for blocks Broadcast
	if err != nil {
		logger.Error("failed to new p2p node", zap.Error(err))
		os.Exit(-1)
	}
	go nB.Run()
	for _, member := range cfg.P2PConfigList[0].Members {
		if err := nB.Join([]string{member}); err != nil {
			logger.Error("Failed to join p2p", zap.Error(err), zap.String("node id", member))
		}
	}
	if cfg.BFTConfig == nil {
		logger.Error("load BFTconfig failed!")
		os.Exit(-1)
	}
	go bftnode.RunbftNode(cfg.BFTConfig, bc, nB, tp)

	nT, err := node.New(cfg.P2PConfigList[1], tp, bc) //use for Tx Broadcast
	if err != nil {
		logger.Error("failed to new p2p node", zap.Error(err))
		os.Exit(-1)
	}
	go nT.Run()
	for _, member := range cfg.P2PConfigList[1].Members {
		if err := nT.Join([]string{member}); err != nil {
			logger.Error("Failed to join Tx p2p", zap.Error(err), zap.String("node id", member))
		}
	}

	if cfg.MonitorCfg != nil {
		monitor.Run(cfg.MonitorCfg, bc)
	}

	if cfg.APIConfig == nil {
		logger.Error("load APIConfig failed!")
		os.Exit(-1)
	}
	api.Start(cfg.APIConfig, bc, tp, nT)

}
