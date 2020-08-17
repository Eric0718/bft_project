package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type CfgInfo struct {
	LogConfig *LogConfigInfo `yaml:"logconfig"`
	//AddressConfig *AddressConfigInfo `yaml:"addressconfig"`
	P2PConfigList []*P2PConfigInfo `yaml:"p2pconfig"`
	//	P2PTxConfig     *P2PTxConfigInfo     `yaml:"p2pTxconfig"`
	//ConsensusConfig *ConsensusConfigInfo `yaml:"consensusconfig"`
	APIConfig *APIConfigInfo `yaml:"apiconfig"`
	BFTConfig *BftConfig     `yaml:"bftconfig"`
}

type LogConfigInfo struct {
	Level      string `yaml:"level"`
	FileName   string `yaml:"filename"`
	MaxSize    int    `yaml:"maxsize"`
	MaxAge     int    `yaml:"maxage"`
	MaxBackups int    `yaml:"maxbackups"`
	Comperss   bool   `yaml:"comperss"`
}

type RPCConfigInfo struct {
	Address   string `yaml:"address"`
	CertFile  string `yaml:"certfile"`
	KeyFile   string `yaml:"keyfile"`
	AdminAddr string `yaml:"adminaddr"`
}

type WEBConfigInfo struct {
	Address string `yaml:"address"`
}

type APIConfigInfo struct {
	Port      string         `yaml:"port"`
	RPCConfig *RPCConfigInfo `yaml:"rpcconfig"`
	WEBConfig *WEBConfigInfo `yaml:"webconfig"`
}

type P2PConfigInfo struct {
	BindPort      int      `yaml:"bindport"`
	BindAddr      string   `yaml:"bindaddr"`
	AdvertiseAddr string   `yaml:"advertiseaddr"`
	NodeName      string   `yaml:"nodename"`
	Members       []string `yaml:"members"`
}

type P2PTxConfigInfo struct {
	BindPort      int      `yaml:"bindport"`
	BindAddr      string   `yaml:"bindaddr"`
	AdvertiseAddr string   `yaml:"advertiseaddr"`
	NodeName      string   `yaml:"nodename"`
	Members       []string `yaml:"members"`
}

type AddressConfigInfo struct {
	QTJAddress   string `yaml:"qtjaddress"`
	DSAddress    string `yaml:"dsaddress"`
	CMAddress    string `yaml:"cmaddress"`
	MinerAddress string `yamil:"mineraddress"`
}

type ConsensusConfigInfo struct {
	Id        int      `yaml:"id"`
	Address   string   `yaml:"address"`
	Peer      string   `yaml:"peer"`
	Peers     []string `yaml:"peers"`
	Join      bool     `yaml:"join"`
	Waldir    string   `yaml:"waldir"`
	Snapdir   string   `yaml:"snapdir"`
	Raftport  int64    `yaml:"raftport"`
	Ds        string   `yaml:"ds"`
	Cm        string   `yaml:"cm"`
	QTJ       string   `yaml:"qtj"`
	SnapCount int64    `yaml:"snapcount"`

	LogFile     string `yaml:"logfile"`
	LogSaveDays int    `yaml:"logsavedays"`
	LogLevel    int    `yaml:"loglevel"`
	LogSaveMode int    `yaml:"logsavemode"`
	LogFileSize int64  `yaml:"logfilesize"`
}

type BftConfig struct {
	NodeNum          uint64   `yaml:"nodenum"`
	Peers            []string `yaml:"peers"`
	HttpAddr         string   `yaml:"httpaddr"`
	NodeAddr         string   `yaml:"nodeaddr"`
	CountAddr        string   `yaml:"countaddr"`
	RpcAddr          string   `yaml:"rpcaddr"`
	RpcPort          string   `yaml:"rpcport"`
	RecPort          string   `yaml:"recport"`
	Join             bool     `yaml:"join"`
	SnapshotCount    uint64   `yaml:"snapshotcount"`
	SnapshotInterval uint64   `yaml:"snapshotinterval"`
	Ds               string   `yaml:"ds"`
	Cm               string   `yaml:"cm"`
	QTJ              string   `yaml:"qtj"`
	LogDir           string   `yaml:"logdir"`
	SnapDir          string   `yaml:"snapdir"`
	LogsDir          string   `yaml:"logsdir"`
	StableDir        string   `yaml:"stabledir"`
	LogFile          string   `yaml:"logfile"`
	LogSaveDays      int      `yaml:"logsavedays"`
	LogLevel         int      `yaml:"loglevel"`
	LogSaveMode      int      `yaml:"logsavemode"`
	LogFileSize      int64    `yaml:"logfilesize"`
}

// Config 配置信息
var Config CfgInfo

// LoadConfig 加载配置信息
func LoadConfig() (*CfgInfo, error) {
	viper.SetConfigName("korthoConf")
	viper.AddConfigPath("./configs/")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg CfgInfo
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	fmt.Printf("%+v\n", cfg)

	return &cfg, nil
}

// func init() {
// 	viper.SetConfigName("kortho.yaml")
// 	viper.AddConfigPath("./configs/")
// 	if err := viper.ReadInConfig(); err != nil {
// 		panic(fmt.Errorf("fatal error config file:%s", err))
// 	}

// 	if err := viper.Unmarshal(&Config); err != nil {
// 		panic(fmt.Errorf("fatal error config file:%s", err))
// 	}
// }
