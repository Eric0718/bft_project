package api

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"kortho/api/message"
	"kortho/config"
	"kortho/logger"
	"kortho/p2p/node"
	"kortho/transaction"
	"kortho/txpool"
	"kortho/types"
	"kortho/util"
	"kortho/util/miscellaneous"
	"net"
	"os"
	"strconv"

	"kortho/blockchain"

	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Greeter rpc服务
type Greeter struct {
	Bc        blockchain.Blockchains
	tp        *txpool.TxPool
	n         node.Node
	Address   string
	tls       tlsInfo
	AdminAddr string
	AdminPriv string
}
type tlsInfo struct {
	certFile string
	keyFile  string
}

func newGreeter(cfg *config.RPCConfigInfo, bc blockchain.Blockchains, tp *txpool.TxPool, n node.Node) *Greeter {
	grpcServ := &Greeter{
		Bc:        bc,
		tp:        tp,
		n:         n,
		Address:   cfg.Address,
		AdminAddr: cfg.AdminAddr,
		tls: tlsInfo{
			certFile: cfg.CertFile,
			keyFile:  cfg.KeyFile,
		},
	}

	return grpcServ
}

// RunRPC run rpc service
func (g *Greeter) RunRPC() {
	lis, err := net.Listen("tcp", g.Address)
	if err != nil {
		logger.Error("net.Listen", zap.Error(err))
		os.Exit(-1)
	}

	// creds, err := credentialg.newServerTLSFromFile(g.tls.certFile, g.tls.keyFile)
	// if err != nil {
	// 	logger.Error("credentialg.newServerTLSFromFile",
	// 		zap.Error(err), zap.String("cert file", g.tls.certFile), zap.String("key file", g.tls.keyFile))
	// 	os.Exit(-1)
	// }
	// server := grpc.NewServer(grpc.Creds(creds), grpc.UnaryInterceptor(ipInterceptor))
	server := grpc.NewServer(grpc.UnaryInterceptor(ipInterceptor))
	message.RegisterGreeterServer(server, g)
	server.Serve(lis)
}

func txToMsgTxAndOrder(tx *transaction.Transaction) (msgTx message.Tx) {
	msgTx.Hash = hex.EncodeToString(tx.Hash)
	msgTx.From = string(tx.From.Bytes())
	msgTx.BlockNum = tx.BlockNumber
	msgTx.Amount = tx.Amount
	msgTx.Nonce = tx.Nonce
	msgTx.To = string(tx.To.Bytes())
	msgTx.Signature = hex.EncodeToString(tx.Signature)
	msgTx.Time = tx.Time
	msgTx.Script = tx.Script
	msgTx.Fee = tx.Fee
	msgTx.Root = tx.Root
	msgTx.Tag = tx.Tag

	if tx.IsOrderTransaction() {
		msgTx.Order = &message.Order{}
		msgTx.Order.Id = string(tx.Order.ID)
		msgTx.Order.Address = tx.Order.Address.String()
		msgTx.Order.Price = tx.Order.Price
		msgTx.Order.Hash = hex.EncodeToString(tx.Order.Hash)
		msgTx.Order.Signature = hex.EncodeToString(tx.Order.Signature)
		msgTx.Order.Ciphertext = string(tx.Order.Ciphertext)
		msgTx.Order.Region = string(tx.Order.Region)
		msgTx.Order.Tradename = string(tx.Order.Tradename)
	}
	return
}

func txToMsgTx(tx *transaction.Transaction) (msgTx message.Tx) {
	msgTx.Hash = hex.EncodeToString(tx.Hash)
	msgTx.From = string(tx.From.Bytes())
	msgTx.Amount = tx.Amount
	msgTx.Nonce = tx.Nonce
	msgTx.To = string(tx.To.Bytes())
	msgTx.Signature = hex.EncodeToString(tx.Signature)
	msgTx.Time = tx.Time
	msgTx.Script = tx.Script
	return msgTx
}

//MsgTxToTx message tx to tx
func MsgTxToTx(msgTx *message.Tx) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{}

	hs, err := hex.DecodeString(msgTx.Hash)
	if err != nil {
		return nil, err
	}
	tx.Hash = hs

	f, er := types.StringToAddress(msgTx.From)
	if er != nil {
		return nil, er
	}
	tx.From = *f

	tx.BlockNumber = msgTx.BlockNum
	tx.Amount = msgTx.Amount
	tx.Nonce = msgTx.Nonce

	t, err := types.StringToAddress(msgTx.To)
	if err != nil {
		return nil, err
	}
	tx.To = *t

	sgt, err := hex.DecodeString(msgTx.Signature)
	if err != nil {
		return nil, err
	}
	tx.Signature = sgt

	tx.Time = msgTx.Time
	tx.Script = msgTx.Script
	tx.Fee = msgTx.Fee
	tx.Root = msgTx.Root
	tx.Tag = msgTx.Tag

	if msgTx.Order != nil && len(msgTx.Signature) > 0 {
		tx.Order.ID = []byte(msgTx.Order.Id)

		ad, err := types.StringToAddress(msgTx.Order.Address)
		if err != nil {
			return nil, err
		}
		tx.Order.Address = *ad

		tx.Order.Price = msgTx.Order.Price

		Ohs, err := hex.DecodeString(msgTx.Order.Hash)
		if err != nil {
			return nil, err
		}
		tx.Order.Hash = Ohs

		sgt, err := hex.DecodeString(msgTx.Order.Signature)
		if err != nil {
			return nil, err
		}
		tx.Order.Signature = sgt

		tx.Order.Ciphertext = []byte(msgTx.Order.Ciphertext)
		tx.Order.Region = msgTx.Order.Region
		tx.Order.Tradename = msgTx.Order.Tradename
	}

	return tx, nil
}

// GetBalance 根据传入的address获取，该address对应的余额
func (g *Greeter) GetBalance(ctx context.Context, in *message.ReqBalance) (*message.ResBalance, error) {

	balance, err := g.Bc.GetBalance([]byte(in.Address))
	if err != nil {
		logger.Error("g.Bc.GetBalance", zap.Error(err), zap.String("address", in.Address))
	}
	return &message.ResBalance{Balnce: balance}, nil
}

// GetBlockByNum 通过块高获取块数据
func (g *Greeter) GetBlockByNum(ctx context.Context, in *message.ReqBlockByNumber) (*message.RespBlock, error) {

	b, err := g.Bc.GetBlockByHeight(in.Height)
	if err != nil {
		logger.Error("g.Bc.GetBlockByHeight", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "height %d not found", in.Height)
	}

	var respdata message.RespBlock
	for _, tx := range b.Transactions {
		tmpTx := txToMsgTxAndOrder(tx)
		respdata.Txs = append(respdata.Txs, &tmpTx)
	}

	respdata.Height = b.Height
	respdata.Hash = hex.EncodeToString(b.Hash)
	respdata.PrevBlockHash = hex.EncodeToString(b.PrevHash)
	respdata.Root = hex.EncodeToString(b.Root)
	respdata.Timestamp = b.Timestamp
	respdata.Version = b.Version
	respdata.Miner = b.Miner.String()
	return &respdata, nil
}

// GetBlockByHash 通过hash获取块数据
func (g *Greeter) GetBlockByHash(ctx context.Context, in *message.ReqBlockByHash) (*message.RespBlock, error) {
	h, _ := hex.DecodeString(in.Hash)
	b, err := g.Bc.GetBlockByHash(h)
	if err != nil {
		logger.Error("g.Bc.GetBlockByHash", zap.Error(err), zap.String("hash", in.Hash))
		return nil, grpc.Errorf(codes.InvalidArgument, "hash %s not found", in.Hash)
	}

	var respdata message.RespBlock
	for _, tx := range b.Transactions {
		tmpTx := txToMsgTxAndOrder(tx)
		respdata.Txs = append(respdata.Txs, &tmpTx)
	}

	respdata.Height = b.Height
	respdata.Hash = hex.EncodeToString(b.Hash)
	respdata.PrevBlockHash = hex.EncodeToString(b.PrevHash)
	respdata.Root = hex.EncodeToString(b.Root)
	respdata.Timestamp = b.Timestamp
	respdata.Version = b.Version
	respdata.Miner = b.Miner.String()
	return &respdata, nil
}

// GetTxsByAddr 获取该address的所有交易
func (g *Greeter) GetTxsByAddr(ctx context.Context, in *message.ReqTx) (*message.ResposeTxs, error) {
	txs, err := g.Bc.GetTransactionByAddr([]byte(in.Address), 0, 9)
	if err != nil {
		logger.Error("g.Bc.GetTransactionByAddr", zap.Error(err))
		return nil, err
	}

	var respData message.ResposeTxs
	for _, tx := range txs {
		tmpTx := txToMsgTxAndOrder(tx)
		respData.Txs = append(respData.Txs, &tmpTx)
	}

	return &respData, nil
}

// GetTxByHash 通过hash获取交易
func (g *Greeter) GetTxByHash(ctx context.Context, in *message.ReqTxByHash) (*message.RespTxByHash, error) {
	hash, err := hex.DecodeString(in.Hash)
	if err != nil {
		logger.Error("Faile to decode hash", zap.Error(err), zap.String("hash", in.Hash))
		return nil, grpc.Errorf(codes.InvalidArgument, "hash %s", in.Hash)
	}

	tx, err := g.Bc.GetTransactionByHash(hash)
	if err != nil {
		logger.Error("Failed to get transaction", zap.Error(err), zap.String("hash", in.Hash))
		//return nil, grpc.Errorf(codes.InvalidArgument, "hash %s", in.Hash)
	} else {
		data := txToMsgTxAndOrder(tx)
		return &message.RespTxByHash{Code: 0, Message: "已上链", Data: &data}, nil
	}

	if g.tp.IsExist(hash) {
		return &message.RespTxByHash{Code: 1, Message: "正在上链"}, nil
	}

	return &message.RespTxByHash{Code: -1, Message: "未上链"}, nil
}

// GetAddressNonceAt 获取该address的nonce，nonce是下次发送交易所需。
func (g *Greeter) GetAddressNonceAt(ctx context.Context, in *message.ReqNonce) (*message.ResposeNonce, error) {
	nonce, err := g.Bc.GetNonce([]byte(in.Address))
	if err != nil {
		logger.Error("g.Bc.GetNonce", zap.Error(err), zap.String("address", in.Address))
		return nil, grpc.Errorf(codes.InvalidArgument, "address %s", in.Address)
	}
	return &message.ResposeNonce{Nonce: nonce}, nil
}

// SendTransaction 发送交易，From向To发起交易，金额为Amount，Nonce是From所需的Nonce
func (g *Greeter) SendTransaction(ctx context.Context, in *message.ReqTransaction) (*message.ResTransaction, error) {
	if in.From == in.To {
		logger.Info("From and To are the same", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	from, err := types.StringToAddress(in.From)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	to, err := types.StringToAddress(in.To)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	priv := util.Decode(in.Priv)
	if len(priv) != 64 {
		logger.Info("private key", zap.String("privateKey", in.Priv))
		return nil, grpc.Errorf(codes.InvalidArgument, "private key:%s", in.Priv)
	}

	tx := transaction.ZNewTransaction(in.Nonce, in.Amount, *from, *to)
	if in.Order != nil {
		if len(in.Order.Address) == types.AddressSize {
			for i, v := range []byte(in.Order.Address) {
				tx.Order.Address[i] = v
			}
			var err error
			tx.Order.ID = []byte(in.Order.Id)
			tx.Order.Price = in.Order.Price
			tx.Order.Ciphertext, err = hex.DecodeString(in.Order.Ciphertext)
			if err != nil {
				logger.Info("hex.DecodeString failed", zap.String("Order.Ciphertext", in.Order.Ciphertext))
				return nil, grpc.Errorf(codes.InvalidArgument, "order.Ciphertext")
			}
			tx.Order.Hash, err = hex.DecodeString(in.Order.Hash)
			if err != nil {
				logger.Info("hex.DecodeString failed", zap.String("Order.Hash", in.Order.Hash))
				return nil, grpc.Errorf(codes.InvalidArgument, "Order.Hash:%s", in.Order.Hash)
			}
			tx.Order.Signature, err = hex.DecodeString(in.Order.Signature)
			if err != nil {
				logger.Info("hex.DecodeString failed", zap.String("Order.Signature", in.Order.Signature))
				return nil, grpc.Errorf(codes.InvalidArgument, "Order.Signature")
			}
			tx.Order.Region = in.Order.Region
			tx.Order.Tradename = in.Order.Tradename
		}
	}
	tx.Sign(priv)

	if err := g.tp.Add(tx, g.Bc); err != nil {
		logger.Error("failed to add txpool", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	g.n.Broadcast(tx)
	hash := hex.EncodeToString(tx.Hash)

	return &message.ResTransaction{Hash: hash}, nil
}

// SendTransactions 批量发送交易，请先看SendTransaction
func (g *Greeter) SendTransactions(ctx context.Context, in *message.ReqTransactions) (*message.RespTransactions, error) {
	var hashList []*message.HashMsg
	for _, v := range in.Txs {
		if v.From == v.To {
			logger.Info("From and To are the same", zap.String("from", v.From), zap.String("to", v.To))
			msg := message.HashMsg{Code: -1, Message: "address cannot be the same", Hash: ""}
			hashList = append(hashList, &msg)
			continue
		}

		from, err := types.StringToAddress(v.From)
		if err != nil {
			logger.Error("Parameters error", zap.String("from", v.From), zap.String("to", v.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: ""}
			hashList = append(hashList, &msg)
			continue
		}

		to, err := types.StringToAddress(v.To)
		if err != nil {
			logger.Error("Parameters error", zap.String("from", v.From), zap.String("to", v.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: ""}
			hashList = append(hashList, &msg)
			continue
		}

		priv := util.Decode(v.Priv)
		if len(priv) != 64 {
			logger.Info("private key", zap.String("privateKey", v.Priv))
			msg := message.HashMsg{Code: -1, Message: "invalid private key", Hash: ""}
			hashList = append(hashList, &msg)
			continue
		}

		tx := transaction.ZNewTransaction(v.Nonce, v.Amount, *from, *to)
		if v.Order != nil {
			if len(v.Order.Address) == types.AddressSize {
				for i, v := range []byte(v.Order.Address) {
					tx.Order.Address[i] = v
				}
				var err error
				tx.Order.ID = []byte(v.Order.Id)
				tx.Order.Price = v.Order.Price
				tx.Order.Ciphertext, err = hex.DecodeString(v.Order.Ciphertext)
				if err != nil {
					logger.Error("hex.DecodeString failed", zap.String("Order.Ciphertext", v.Order.Ciphertext))
					msg := message.HashMsg{Code: -1, Message: "invalid order ciphertext", Hash: ""}
					hashList = append(hashList, &msg)
					continue
				}
				tx.Order.Hash, err = hex.DecodeString(v.Order.Hash)
				if err != nil {
					logger.Info("hex.DecodeString failed", zap.String("Order.Hash", v.Order.Hash))
					msg := message.HashMsg{Code: -1, Message: "invalid order hash", Hash: ""}
					hashList = append(hashList, &msg)
					continue
				}
				tx.Order.Signature, err = hex.DecodeString(v.Order.Signature)
				if err != nil {
					logger.Info("hex.DecodeString failed", zap.String("Order.Signature", v.Order.Signature))
					msg := message.HashMsg{Code: -1, Message: "invalid order signature", Hash: ""}
					hashList = append(hashList, &msg)
					continue
				}
				tx.Order.Region = v.Order.Region
				tx.Order.Tradename = v.Order.Tradename
			}
		}
		tx.Sign(priv)

		if err := g.tp.Add(tx, g.Bc); err != nil {
			logger.Error("failed to add txpool", zap.Error(err))
			msg := message.HashMsg{Code: -1, Message: "failed to add txpool", Hash: hex.EncodeToString(tx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		g.n.Broadcast(tx)
		msg := message.HashMsg{Code: 0, Message: "ok", Hash: hex.EncodeToString(tx.Hash)}
		hashList = append(hashList, &msg)
	}

	return &message.RespTransactions{HashList: hashList}, nil
}

// SendSignedTransaction 将完整的交易发送到交易池，等待上链
func (g *Greeter) SendSignedTransaction(ctx context.Context, in *message.ReqSignedTransaction) (*message.RespSignedTransaction, error) {

	if in.From == in.To {
		logger.Info("From and To are the same", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	from, err := types.StringToAddress(in.From)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	to, err := types.StringToAddress(in.To)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	// tx := transaction.ZNewTransaction(in.Nonce, in.Amount, *from, *to)
	// tx.Signature = in.Signature

	tx := &transaction.Transaction{
		From:      *from,
		To:        *to,
		Nonce:     in.Nonce,
		Amount:    in.Amount,
		Time:      in.Time,
		Hash:      in.Hash,
		Signature: in.Signature,
	}

	if !tx.Verify() {
		logger.Error("failed to verify transation", zap.Error(errors.New("signature verification failed")))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	if err := g.tp.Add(tx, g.Bc); err != nil {
		logger.Error("failed to add txpool", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	g.n.Broadcast(tx)
	return &message.RespSignedTransaction{Hash: hex.EncodeToString(tx.Hash)}, nil
}

// SendSignedTransactions 将完整的交易列表发送到交易池，等待上链
func (g *Greeter) SendSignedTransactions(ctx context.Context, in *message.ReqSignedTransactions) (*message.RespSignedTransactions, error) {
	var hashList []*message.HashMsg
	for _, reqTx := range in.Txs {
		if reqTx.From == reqTx.To {
			logger.Info("From and To are the same", zap.String("from", reqTx.From), zap.String("to", reqTx.To))
			msg := message.HashMsg{Code: -1, Message: "address cannot be the same", Hash: hex.EncodeToString(reqTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		from, err := types.StringToAddress(reqTx.From)
		if err != nil {
			logger.Error("Parameters error", zap.String("from", reqTx.From), zap.String("to", reqTx.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: hex.EncodeToString(reqTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		to, err := types.StringToAddress(reqTx.To)
		if err != nil {
			logger.Error("Parameters error", zap.String("from", reqTx.From), zap.String("to", reqTx.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: hex.EncodeToString(reqTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		tx := &transaction.Transaction{
			From:      *from,
			To:        *to,
			Nonce:     reqTx.Nonce,
			Amount:    reqTx.Amount,
			Time:      reqTx.Time,
			Hash:      reqTx.Hash,
			Signature: reqTx.Signature,
		}

		if !tx.Verify() {
			logger.Error("failed to verify transation", zap.Error(errors.New("signature verification failed")))
			msg := message.HashMsg{Code: -1, Message: "sign verification failed", Hash: hex.EncodeToString(reqTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		if err := g.tp.Add(tx, g.Bc); err != nil {
			logger.Error("failed to add txpool", zap.Error(err))
			msg := message.HashMsg{Code: -1, Message: "failed to add txpool", Hash: hex.EncodeToString(reqTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}
		g.n.Broadcast(tx)

		msg := message.HashMsg{Code: 0, Message: "ok", Hash: hex.EncodeToString(reqTx.Hash)}
		hashList = append(hashList, &msg)
	}

	return &message.RespSignedTransactions{HashList: hashList}, nil
}

// CreateContract 创建代币合约，Symbol是代币名称，Total是发行总量，Fee所需交易费。
func (g *Greeter) CreateContract(ctx context.Context, in *message.ReqTokenCreate) (*message.RespTokenCreate, error) {

	if in.From == in.To {
		logger.Info("From and To are the same", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	from, err := types.StringToAddress(in.From)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	to, err := types.StringToAddress(in.To)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	priv := util.Decode(in.Priv)
	if len(priv) != 64 {
		logger.Info("private key", zap.String("privateKey", in.Priv))
		return nil, grpc.Errorf(codes.InvalidArgument, "private key:%s", in.Priv)
	}

	//"new \"abc\" 1000000000"
	At := strconv.FormatUint(in.Total, 10)
	//script := "new" + '\"' + in.Symbol +'\"' + in.Total

	script := fmt.Sprintf("new \"%s\" %s", in.Symbol, At)

	//tx := transaction.Newtoken(in.Nonce, uint64(500001), in.Fee, *from, *to, script)
	//TODO:transaction.WithToken(in.Fee, script, []byte{}) 把空的字节切片换成root
	tx := transaction.ZNewTransaction(in.Nonce, uint64(500001), *from, *to,
		transaction.WithToken(in.Fee, script, []byte{}))
	tx.Sign(priv)

	if err := g.tp.Add(tx, g.Bc); err != nil {
		logger.Error("g.tp.Add", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	g.n.Broadcast(tx)
	hash := hex.EncodeToString(tx.Hash)

	return &message.RespTokenCreate{Hash: hash}, nil
}

// MintToken 创建合约后调用，参数与CreateContract所需完全一致
func (g *Greeter) MintToken(ctx context.Context, in *message.ReqTokenCreate) (*message.RespTokenCreate, error) {

	if in.From == in.To {
		logger.Info("From and To are the same", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	from, err := types.StringToAddress(in.From)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	to, err := types.StringToAddress(in.To)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	priv := util.Decode(in.Priv)
	if len(priv) != 64 {
		logger.Info("private key", zap.String("privateKey", in.Priv))
		return nil, grpc.Errorf(codes.InvalidArgument, "private key:%s", in.Priv)
	}

	//"new \"abc\" 1000000000"
	At := strconv.FormatUint(in.Total, 10)
	//script := "new" + '\"' + in.Symbol +'\"' + in.Total

	script := fmt.Sprintf("mint \"%s\" %s", in.Symbol, At)
	//tx := transaction.Newtoken(in.Nonce, uint64(500001), in.Fee, *from, *to, script)
	//TODO:transaction.WithToken(in.Fee, script, []byte{}) 把空的字节切片换成root
	tx := transaction.ZNewTransaction(in.Nonce, uint64(500001), *from, *to,
		transaction.WithToken(in.Fee, script, []byte{}))

	tx.Sign(priv)

	if err := g.tp.Add(tx, g.Bc); err != nil {
		logger.Error("g.tp.Add", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	g.n.Broadcast(tx)
	hash := hex.EncodeToString(tx.Hash)

	return &message.RespTokenCreate{Hash: hash}, nil
}

// SendToken 发送代币交易
func (g *Greeter) SendToken(ctx context.Context, in *message.ReqTokenTransaction) (*message.RespTokenTransaction, error) {
	if in.From == in.To {
		logger.Info("From and To are the same", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	from, err := types.StringToAddress(in.From)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	to, err := types.StringToAddress(in.To)
	if err != nil {
		logger.Error("Parameters error", zap.String("from", in.From), zap.String("to", in.To))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s,to:%s", in.From, in.To)
	}

	priv := util.Decode(in.Priv)
	if len(priv) != 64 {
		logger.Info("in.Priv", zap.String("in.Priv", in.Priv))
		return nil, grpc.Errorf(codes.InvalidArgument, "priv:%s", in.Priv)
	}

	At := strconv.FormatUint(in.TokenAmount, 10)
	//"transfer \"abc\" 10 \"to\""
	script := fmt.Sprintf("transfer \"%s\" %s \"%s\"", in.Symbol, At, in.To)
	//tx = tx.NewTransaction(in.Nonce, in.Amount, from, to, script)

	root, err := g.Bc.GetTokenRoot(from.String(), script)
	if err != nil {
		logger.Error("Failed to get token root", zap.String("from", from.String()),
			zap.String("script", script))
	}
	tx := transaction.ZNewTransaction(in.Nonce, 500001, *from, *to, transaction.WithToken(in.Fee, script, root))
	tx.Sign(priv)

	if err := g.tp.Add(tx, g.Bc); err != nil {
		logger.Error("Failed to add transaction", zap.Error(err))
		return nil, grpc.Errorf(codes.InvalidArgument, "data error")
	}

	g.n.Broadcast(tx)
	hash := hex.EncodeToString(tx.Hash)

	return &message.RespTokenTransaction{Hash: hash}, nil
}

// GetBalanceToken 获取address对应代币的余额，Symbol为代币名称
func (g *Greeter) GetBalanceToken(ctx context.Context, in *message.ReqTokenBalance) (*message.RespTokenBalance, error) {
	balance, err := g.Bc.GetTokenBalance([]byte(in.Address), []byte(in.Symbol))
	if err != nil {
		logger.Error("g.Bc.GetTokenBalance", zap.Error(err), zap.String("address", in.Address), zap.String("symbol", in.Symbol))
		return nil, grpc.Errorf(codes.InvalidArgument, "symbol:\"%s\",address:%s", in.Symbol, in.Address)
	}
	return &message.RespTokenBalance{Balnce: balance}, nil
}

// CreateAddr 在线创建地址和私钥
func (g *Greeter) CreateAddr(ctx context.Context, in *message.ReqCreateAddr) (*message.RespCreateAddr, error) {
	wallet := types.NewWallet()
	return &message.RespCreateAddr{Address: wallet.Address, Privkey: util.Encode(wallet.PrivateKey)}, nil
}

// GetMaxBlockNumber 获取最大的块号
func (g *Greeter) GetMaxBlockNumber(ctx context.Context, in *message.ReqMaxBlockNumber) (*message.RespMaxBlockNumber, error) {

	maxNumber, err := g.Bc.GetMaxBlockHeight()
	if err != nil {
		logger.Error("g.Bc.GetMaxBlockHeight", zap.Error(err))
		//TODO:如何保证数据库中确实是不存在块高？
		return &message.RespMaxBlockNumber{MaxNumber: 0}, nil
	}
	return &message.RespMaxBlockNumber{MaxNumber: maxNumber}, nil
}

// GetAddrByPriv 通过私钥获取地址
func (g *Greeter) GetAddrByPriv(ctx context.Context, in *message.ReqAddrByPriv) (*message.RespAddrByPriv, error) {
	privBytes := util.Decode(in.Priv)
	if len(privBytes) != 64 {
		logger.Error("private key", zap.String("in.Priv", in.Priv))
		return nil, grpc.Errorf(codes.InvalidArgument, "wrong private key")
	}
	addr := util.PubtoAddr(privBytes[32:])
	return &message.RespAddrByPriv{Addr: addr}, nil
}

// SignOrd 在线对订单进行签名
func (g *Greeter) SignOrd(ctx context.Context, in *message.ReqSignOrd) (*message.RespSignOrd, error) {
	if in.Order == nil || len(in.Order.Id) == 0 || len(in.Order.Address) == 0 || len(in.Order.Ciphertext) == 0 ||
		in.Order.Price == 0 || len(in.Priv) == 0 {
		logger.Info("order data", zap.Any("order", *in))
		return nil, grpc.Errorf(codes.InvalidArgument, "Parameter is not complete")
	}

	id, err := hex.DecodeString(in.Order.Id)
	if err != nil {
		logger.Info("hex.DecodeString", zap.String("order.Id", in.Order.Id))
		return nil, grpc.Errorf(codes.InvalidArgument, "id:%s", in.Order.Id)
	}

	price := miscellaneous.E64func(in.Order.Price)
	ciphertext, err := hex.DecodeString(in.Order.Ciphertext)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Order.Ciphertext")
	}

	var hashBytes []byte
	hashBytes = append(hashBytes, price...)
	hashBytes = append(hashBytes, id...)
	hashBytes = append(hashBytes, ciphertext...)
	hashBytes = append(hashBytes, []byte(in.Order.Address)...)
	hash32 := sha3.Sum256(hashBytes)
	Hash := hex.EncodeToString(hash32[:])
	pri := ed25519.PrivateKey(util.Decode(in.Priv))
	signatures := ed25519.Sign(pri, hash32[:])
	Signature := hex.EncodeToString(signatures)
	return &message.RespSignOrd{Hash: Hash, Signature: Signature}, nil
}

// SendFreezeTransactions 冻结To账户Amount数额的余额
func (g *Greeter) SendFreezeTransactions(ctx context.Context, in *message.ReqSignedTransactions) (*message.RespSignedTransactions, error) {
	addr, err := types.StringToAddress(g.AdminAddr)
	if err != nil {
		logger.Error("Faile to change from", zap.String("from", g.AdminAddr))
		return nil, grpc.Errorf(codes.InvalidArgument, "from:%s", g.AdminAddr)
	}
	var hashList []*message.HashMsg
	for _, freezeTx := range in.Txs {
		to, err := types.StringToAddress(freezeTx.To)
		if err != nil {
			logger.Error("faile to verify address", zap.Error(err), zap.String("address", freezeTx.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: hex.EncodeToString(freezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		// hash, err := hex.DecodeString(freezeTx.Hash)
		// if err != nil {
		// 	logger.Error("failed to decode hash", zap.Error(err), zap.String("address", freezeTx.Address))
		// 	continue
		// }

		hash := freezeTx.Hash

		// signature, err := hex.DecodeString(freezeTx.Signature)
		// if err != nil {
		// 	logger.Error("failed to decode signature", zap.Error(err), zap.String("address", freezeTx.Address))
		// 	continue
		// }

		signature := freezeTx.Signature

		nonce := freezeTx.Nonce
		tx := &transaction.Transaction{
			From:      *addr,
			To:        *to,
			Nonce:     nonce,
			Time:      freezeTx.Time,
			Amount:    freezeTx.Amount,
			Hash:      hash,
			Signature: signature,
			Tag:       transaction.FreezeTag,
		}
		if !tx.Verify() {
			logger.Error("failed to verify transaction", zap.String("to", tx.To.String()),
				zap.Uint64("amount", freezeTx.Amount))
			msg := message.HashMsg{Code: -1, Message: "sign verification failed", Hash: hex.EncodeToString(freezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		//tx := transaction.ZNewTransaction(nonce, freezeTx.Amount, *addr, *to, transaction.WithFreezeBalance())

		if err := g.tp.Add(tx, g.Bc); err != nil {
			logger.Error("Failed to add txpool", zap.Error(err), zap.String("to", tx.To.String()),
				zap.Uint64("nonce", nonce), zap.Uint64("amount", freezeTx.Amount))
			msg := message.HashMsg{Code: -1, Message: "invalid parameter", Hash: hex.EncodeToString(freezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		g.n.Broadcast(tx)
		msg := message.HashMsg{Code: 0, Message: "ok", Hash: hex.EncodeToString(freezeTx.Hash)}
		hashList = append(hashList, &msg)
	}
	return &message.RespSignedTransactions{HashList: hashList}, nil
}

// SendUnfreezeTransactions 解冻address的amount数额的金额
func (g *Greeter) SendUnfreezeTransactions(ctx context.Context, in *message.ReqSignedTransactions) (*message.RespSignedTransactions, error) {
	addr, err := types.StringToAddress(g.AdminAddr)
	if err != nil {
		logger.Error("Faile to change address", zap.String("address", g.AdminAddr))
		return nil, grpc.Errorf(codes.InvalidArgument, "admin address:%s", g.AdminAddr)
	}
	var hashList []*message.HashMsg
	for _, unfreezeTx := range in.Txs {
		to, err := types.StringToAddress(unfreezeTx.To)
		if err != nil {
			logger.Error("Faile to Verify address", zap.Error(err), zap.String("to", unfreezeTx.To))
			msg := message.HashMsg{Code: -1, Message: "invalid address", Hash: hex.EncodeToString(unfreezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		// hash, err := hex.DecodeString(unfreezeTx.Hash)
		// if err != nil {
		// 	logger.Error("failed to decode hash", zap.Error(err), zap.String("address", unfreezeTx.Address))
		// 	continue
		// }
		hash := unfreezeTx.Hash

		// signature, err := hex.DecodeString(unfreezeTx.Signature)
		// if err != nil {
		// 	logger.Error("failed to decode signature", zap.Error(err), zap.String("address", unfreezeTx.Address))
		// 	continue
		// }

		signature := unfreezeTx.Signature

		nonce := unfreezeTx.Nonce
		tx := &transaction.Transaction{
			From:      *addr,
			To:        *to,
			Nonce:     nonce,
			Time:      unfreezeTx.Time,
			Amount:    unfreezeTx.Amount,
			Hash:      hash,
			Signature: signature,
			Tag:       transaction.UnfreezeTag,
		}

		if !tx.Verify() {
			logger.Error("failed to verify transaction", zap.String("to", tx.To.String()),
				zap.Uint64("amount", unfreezeTx.Amount))
			msg := message.HashMsg{Code: -1, Message: "sign verification failed", Hash: hex.EncodeToString(unfreezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		if err := g.tp.Add(tx, g.Bc); err != nil {
			logger.Error("Failed to add txpool", zap.Error(err), zap.String("to", tx.To.String()),
				zap.Uint64("nonce", nonce), zap.Uint64("amount", unfreezeTx.Amount))
			msg := message.HashMsg{Code: -1, Message: "invalid parameter", Hash: hex.EncodeToString(unfreezeTx.Hash)}
			hashList = append(hashList, &msg)
			continue
		}

		g.n.Broadcast(tx)
		msg := message.HashMsg{Code: 0, Message: "ok", Hash: hex.EncodeToString(unfreezeTx.Hash)}
		hashList = append(hashList, &msg)
	}

	return &message.RespSignedTransactions{HashList: hashList}, nil
}

// GetFreezeBalance 获取address已冻结的的金额
func (g *Greeter) GetFreezeBalance(ctx context.Context, in *message.ReqGetFreezeBal) (*message.RespGetFreezeBal, error) {
	var resp message.RespGetFreezeBal

	for _, addrStr := range in.AddressList {
		var balance uint64
		var result message.FreezeBalance
		address, err := types.StringToAddress(addrStr)
		if err != nil {
			result.State = -1
			logger.Error("Failed to verify address", zap.String("address", addrStr))
		} else {
			result.State = 0
			balance, _ = g.Bc.GetFreezeBalance(address.Bytes())
		}

		result.Address = addrStr
		result.Balance = balance
		resp.Results = append(resp.Results, &result)
	}

	return &resp, nil
}
