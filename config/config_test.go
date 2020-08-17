package config

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, _ := LoadConfig()
	t.Logf("%+v\n", cfg)
	t.Logf("%+v\n", cfg.LogConfig)

	// if cfg.ConsensusConfig == nil {
	// 	t.Log("raft is nil")
	// }

	if cfg.BFTConfig == nil {
		t.Fatal("bft is nil")
	} else {
		fmt.Printf("%+v", *(cfg.BFTConfig))
	}

}
