package monitor

import (
	"kortho/config"
	"testing"
)

func Test_getLeader(t *testing.T) {
	addr, err := GetLeader("106.12.186.114:6363")
	if err != nil {
		t.Logf("GetLeader error:%v", err)
	} else {
		t.Logf("leader:%v", addr)
	}
}

func Test_getStartBlock(t *testing.T) {
	m := &monitor{
		peer:             "182.61.186.204:9501",
		startBlockHeight: 1,
	}

	res, err := m.getStartBlock()
	if err != nil {
		t.Logf("error:%v", err)
	}
	t.Logf("result:height=%v,hash=%v", res.Height, res.Hash)
}

func Test_getOtherNodesBlockByHeight(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Logf("load config failed:%v", err)
	}
	m := &monitor{
		peers: cfg.MonitorCfg.MPeers,
	}

	err = m.getOtherNodesBlockByHeight(1)
	if err != nil {
		t.Logf("error:%v", err)
	}
}
