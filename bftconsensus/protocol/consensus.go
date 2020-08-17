//This package implements to new a raft node and build cluster connections between nodes.
package protocol

import (
	"kortho/logger"
	"net"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"go.uber.org/zap"
)

//new a raft node
func New(cfg *Config, fsm raft.FSM) (*node, error) {
	trans, err := newRaftTransport(cfg.Address)
	if err != nil {
		logger.Error("newRaftTransport error", zap.Error(err), zap.String("node addr", cfg.Address))
		return nil, err
	}

	raftCfg := raft.DefaultConfig()
	raftCfg.SnapshotThreshold = cfg.SnapshotThreshold
	raftCfg.SnapshotInterval = 5 * time.Minute
	raftCfg.LocalID = raft.ServerID(cfg.Address)
	raftCfg.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  cfg.LogDir,
		Level: hclog.LevelFromString("error"),
	})
	snaps, err := raft.NewFileSnapshotStore(cfg.SnapDir, 1, nil)
	if err != nil {
		logger.Error("newRaftTransport error", zap.Error(err))
		return nil, err
	}
	logs, err := raftboltdb.NewBoltStore(cfg.LogsDir)
	if err != nil {
		logger.Error("NewBoltStore logs error", zap.Error(err))
		return nil, err
	}
	stable, err := raftboltdb.NewBoltStore(cfg.StableDir)
	if err != nil {
		logger.Error("NewBoltStore stable error", zap.Error(err))
		return nil, err
	}
	r, err := raft.NewRaft(raftCfg, fsm, logs, stable, snaps, trans)
	if err != nil {
		logger.Error("NewRaft error", zap.Error(err))
		return nil, err
	}
	if cfg.Join { //only leader do this and start a cluster at first time.
		if err := r.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftCfg.LocalID,
					Address: raft.ServerAddress(cfg.Address),
				},
			},
		}); err.Error() != nil {
			logger.Error("BootstrapCluster error", zap.Error(err.Error()))
			return nil, err.Error()
		}
	}
	return &node{fsm: fsm, Raft: r}, nil
}

//build raft transport
func newRaftTransport(address string) (*raft.NetworkTransport, error) {
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	return raft.NewTCPTransport(addr.String(), addr, 10, 10*time.Second, nil)
}
