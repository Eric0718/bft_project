package txpool

import (
	"bytes"
	"container/heap"
	"crypto/sha256"
	"encoding/json"
	"kortho/block"
	"kortho/blockchain"
	"kortho/logger"
	"kortho/transaction"
	"kortho/types"
	"kortho/util"
	"kortho/util/merkle"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// ReadyTotal 可以出块的交易数量
	ReadyTotal = 500

	// NonceLimits 同一地址在交易中可以过量储存的数量
	NonceLimits = 200

	// PoolListRange 交易池的最大容量
	PoolListRange = 3000
)

const (
	// MinAmount 交易的最小金额
	MinAmount = 500000
)

// QTJPubKey 趣淘净的地址
var QTJPubKey []byte

// CheckBlock 用来验证block的数据结构
type CheckBlock struct {
	Nodeid string
	Height uint64
	Hash   []byte
	Code   bool
}

// TxPool 交易池结构体
type TxPool struct {
	Mutex sync.RWMutex
	List  *TxHeap
	Idhc  map[string]CheckBlock
}

type stateInfo struct {
	nonce   uint64
	balance uint64
}

// New 新建交易池，并传入趣淘鲸的地址
func New(address string) (*TxPool, error) {
	pool := &TxPool{
		List: new(TxHeap),
		Idhc: make(map[string]CheckBlock),
	}
	heap.Init(pool.List)

	//TODO:判断Address是否符合条件
	addr, err := types.StringToAddress(address)
	if err != nil {
		logger.Error("failed to verify address", zap.String("address", address))
		return nil, err
	}
	QTJPubKey = addr.ToPublicKey()
	return pool, nil
}

// Add 添加交易到交易池
func (pool *TxPool) Add(tx *transaction.Transaction, bc blockchain.Blockchains) error {
	pool.Mutex.Lock()
	defer pool.Mutex.Unlock()

	if pool.List.Len() > PoolListRange {
		return errtxoutrange
	}

	if !verify(*tx, bc) {
		return errtx
	}

	if !pool.List.check(tx.From, tx.Nonce) {
		return errtomuch
	}

	heap.Push(pool.List, tx)

	logger.Info("add info", zap.String("from", tx.From.String()), zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount))
	return nil
}

// IsExist 线程池中是否存在该hash对应的交易
func (pool *TxPool) IsExist(hash []byte) bool {
	pool.Mutex.RLock()
	defer pool.Mutex.RUnlock()

	for _, tx := range *pool.List {
		if bytes.Compare(hash, tx.Hash) == 0 {
			return true
		}
	}
	return false
}

// Pending 从交易池中取出可以上链的交易
func (pool *TxPool) Pending(Bc blockchain.Blockchains) (readyTxs []*transaction.Transaction) {
	logger.Info("Into Pending...", zap.Int("pool list length", pool.List.Len()))
	pool.Mutex.Lock()
	defer pool.Mutex.Unlock()
	var err error
	var noReadyTxs []*transaction.Transaction
	nonceMap := make(map[string]uint64)
	frozenBalMap := make(map[string]uint64)
	avaliableBalMap := make(map[string]uint64)

	for pool.List.Len() != 0 && len(readyTxs) < ReadyTotal {
		var ok bool
		var address types.Address
		var avaliableBal, frozenBal, nonce uint64
		tx := heap.Pop(pool.List).(*transaction.Transaction)

		if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
			address = tx.To
		} else {
			address = tx.From
		}

		if avaliableBal, ok = avaliableBalMap[address.String()]; !ok {
			balance, err := Bc.GetBalance(address.Bytes())
			if err != nil {
				logger.Error("failed to get balance", zap.Error(err), zap.String("address", address.String()))
				return
			}

			frozenBal, err = Bc.GetFreezeBalance(address.Bytes())
			if err != nil {
				logger.Error("failed to get frozen amount", zap.Error(err), zap.String("address", address.String()))
				return
			}

			if util.Uint64SubOverflow(balance, frozenBal) {
				logger.Error("balance is less than the frozen amount", zap.String("from", address.String()), zap.Uint64("balance", balance),
					zap.Uint64("frozen amount", frozenBal))
				return
			}
			avaliableBal = balance - frozenBal
		}

		if frozenBal, ok = avaliableBalMap[address.String()]; !ok {
			frozenBal, err = Bc.GetFreezeBalance(address.Bytes())
			if err != nil {
				logger.Error("failed to get frozen amount", zap.Error(err), zap.String("address", address.String()))
				return
			}
		}

		if nonce, ok = nonceMap[tx.From.String()]; !ok {
			nonce, err = Bc.GetNonce(tx.From.Bytes())
			if err != nil {
				logger.Error("failed to get nonce", zap.Error(err), zap.String("from", tx.From.String()))
				return
			}
		}

		if tx.IsUnfreezeTransaction() {
			//TODO:是解锁交易的处理情况
			if nonce == tx.Nonce && !util.Uint64SubOverflow(frozenBal, tx.Amount) {
				//if tx.Amount > frozenBal && nonce == tx.Nonce {
				nonce++
				nonceMap[tx.From.String()] = nonce
				frozenBalMap[address.String()] = frozenBal - tx.Amount
				avaliableBalMap[address.String()] = avaliableBal + tx.Amount
				readyTxs = append(readyTxs, tx)
				continue
			} else if tx.Nonce > nonce && nonce+NonceLimits < tx.Nonce { //TODO:要避免无法上链的tx越积越多,可以设置nonce的差距
				noReadyTxs = append(noReadyTxs, tx)
				continue
			}

			logger.Error("nonce or amount error", zap.String("from", tx.From.String()), zap.Uint64("current nonce", nonce),
				zap.Uint64("tx nonce", tx.Nonce), zap.Uint64("avaliable balance", avaliableBal), zap.Uint64("amount", tx.Amount))
		} else {
			if nonce == tx.Nonce && !util.Uint64SubOverflow(avaliableBal, tx.Amount, tx.Fee) {
				//if balance >= tx.Amount+tx.Fee && nonce == tx.Nonce {
				nonce++
				nonceMap[tx.From.String()] = nonce
				avaliableBalMap[address.String()] = avaliableBal - tx.Amount - tx.Fee
				if tx.IsFreezeTransaction() {
					frozenBalMap[address.String()] = frozenBal + tx.Amount
				}
				readyTxs = append(readyTxs, tx)
				continue
			} else if tx.Nonce > nonce && nonce+NonceLimits < tx.Nonce { //TODO:要避免无法上链的tx越积越多,可以设置nonce的差距
				noReadyTxs = append(noReadyTxs, tx)
				continue
			}
			logger.Error("nonce or amount error", zap.String("from", tx.From.String()), zap.Uint64("current nonce", nonce),
				zap.Uint64("tx nonce", tx.Nonce), zap.Uint64("avvaliable balance", avaliableBal), zap.Uint64("amount", tx.Amount))
		}
	}

	//push the no ready txs into pool list
	for _, tx := range noReadyTxs {
		pool.List.Push(tx)
	}
	logger.Info("end to pending transaction")
	return
}

func verify(tx transaction.Transaction, bc blockchain.Blockchains) bool {
	// if tx.IsCoinBaseTransaction() {
	// 	//1、检查to
	// 	if !tx.To.Verify() {
	// 		logger.Info("failed to verify address", zap.String("to", tx.To.String()))
	// 		return false
	// 	}
	// } else if tx.IsFreezeTransaction() {
	// 	//1、检查from
	// 	if !tx.From.Verify() {
	// 		logger.Info("faile to verify address", zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	//2、检查to
	// 	if !tx.To.Verify() {
	// 		logger.Info("faile to verify address", zap.String("to", tx.To.String()))
	// 		return false
	// 	}

	// 	//3、检查nonce
	// 	nonce, err := bc.GetNonce(tx.From.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get nonce", zap.Error(err), zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	if tx.Nonce < nonce {
	// 		logger.Info("failed to verify nonce", zap.String("from", tx.From.String()),
	// 			zap.Uint64("transaction nonce", tx.Nonce), zap.Uint64("nonce", nonce))
	// 		return false
	// 	}

	// 	//4、检查to余额
	// 	balance, err := bc.GetBalance(tx.To.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get balance", zap.Error(err), zap.String("to", tx.To.String()))
	// 		return false
	// 	}
	// 	freezeBal, err := bc.GetFreezeBalance(tx.To.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get freezebalance", zap.Error(err), zap.String("to", tx.To.String()))
	// 	}
	// 	if tx.Amount < MinAmount || tx.Amount+freezeBal > balance {
	// 		logger.Info("failed to verify amount", zap.String("to", tx.To.String()),
	// 			zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount), zap.Uint64("unlockbalance", balance))
	// 		return false
	// 	}

	// 	//5、验证签名
	// 	if !tx.Verify() {
	// 		logger.Info("failed to verify transaction",
	// 			zap.String("from", tx.From.String()), zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount))
	// 		return false
	// 	}
	// } else if tx.IsUnfreezeTransaction() {
	// 	//1、检查from
	// 	if !tx.From.Verify() {
	// 		logger.Info("faile to verify address", zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	//2、检查to
	// 	if !tx.To.Verify() {
	// 		logger.Info("faile to verify address", zap.String("to", tx.To.String()))
	// 		return false
	// 	}

	// 	//3、检查nonce
	// 	nonce, err := bc.GetNonce(tx.From.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get nonce", zap.Error(err), zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	if tx.Nonce < nonce {
	// 		logger.Info("failed to verify nonce", zap.String("from", tx.From.String()),
	// 			zap.Uint64("transaction nonce", tx.Nonce), zap.Uint64("nonce", nonce))
	// 		return false
	// 	}

	// 	//4、检查to余额
	// 	frozenBal, err := bc.GetFreezeBalance(tx.To.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get frozen balance", zap.Error(err), zap.String("to", tx.To.String()))
	// 	}

	// 	if tx.Amount < MinAmount || tx.Amount > frozenBal {
	// 		logger.Info("failed to verify amount", zap.String("to", tx.To.String()), zap.String("to", tx.To.String()),
	// 			zap.Uint64("amount", tx.Amount), zap.Uint64("frozen balance", frozenBal))
	// 		return false
	// 	}

	// 	//5、验证签名
	// 	if !tx.Verify() {
	// 		logger.Info("failed to verify transaction",
	// 			zap.String("from", tx.From.String()), zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount))
	// 		return false
	// 	}
	// } else {
	// 	//1、检查from
	// 	if !tx.From.Verify() {
	// 		logger.Info("faile to verify address", zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	//2、检查to
	// 	if !tx.To.Verify() {
	// 		logger.Info("failed to verify address", zap.String("to", tx.To.String()))
	// 		return false
	// 	}

	// 	//3、检查nonce
	// 	nonce, err := bc.GetNonce(tx.From.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get nonce", zap.Error(err), zap.String("from", tx.From.String()))
	// 		return false
	// 	}

	// 	if tx.Nonce < nonce {
	// 		logger.Info("failed to verify nonce", zap.String("from", tx.From.String()),
	// 			zap.Uint64("transaction nonce", tx.Nonce), zap.Uint64("nonce", nonce))
	// 		return false
	// 	}

	// 	//4、检查from余额
	// 	balance, err := bc.GetBalance(tx.From.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get balance", zap.Error(err), zap.String("from", tx.From.String()))
	// 		return false
	// 	}
	// 	freezeBal, err := bc.GetFreezeBalance(tx.From.Bytes())
	// 	if err != nil {
	// 		logger.Error("failed to get freezeBal", zap.Error(err), zap.String("from", tx.From.String()))
	// 		return false
	// 	}
	// 	unLockBal := balance - freezeBal
	// 	if tx.Amount < MinAmount || tx.Amount > unLockBal {
	// 		logger.Info("failed to verify amount", zap.String("from", tx.From.String()),
	// 			zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount), zap.Uint64("unlockbalance", unLockBal))
	// 		return false
	// 	}

	// 	if tx.IsTokenTransaction() && (tx.Fee < MinAmount || tx.Fee+tx.Amount > unLockBal) {
	// 		return false
	// 	}

	// 	//5、验证签名
	// 	if !tx.Verify() {
	// 		logger.Info("failed to verify transaction",
	// 			zap.String("from", tx.From.String()), zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount))
	// 		return false
	// 	}

	// 	//6、检查order
	// 	if tx.IsOrderTransaction() {
	// 		ok := ed25519.Verify(ed25519.PublicKey(QTJPubKey), tx.Order.Hash, tx.Order.Signature)
	// 		if ok != true {
	// 			logger.Info("failed to verify order", zap.String("id", string(tx.Order.ID)))
	// 			return false
	// 		}
	// 	}
	// }

	//1、检查from
	if !tx.IsCoinBaseTransaction() && !tx.From.Verify() {
		logger.Info("faile to verify address", zap.String("from", tx.From.String()))
		return false
	}

	//2、检查to
	if !tx.To.Verify() {
		logger.Info("faile to verify address", zap.String("to", tx.To.String()))
		return false
	}

	//3、检查nonce
	if !tx.IsCoinBaseTransaction() {
		nonce, err := bc.GetNonce(tx.From.Bytes()) //TODO:如果from为空值的话会卡主
		if err != nil {
			logger.Error("failed to get nonce", zap.Error(err), zap.String("from", tx.From.String()))
			return false
		}

		if tx.Nonce < nonce {
			logger.Info("failed to verify nonce", zap.String("from", tx.From.String()),
				zap.Uint64("transaction nonce", tx.Nonce), zap.Uint64("nonce", nonce))
			return false
		}
	}

	//4、验证签名
	if !tx.IsCoinBaseTransaction() && !tx.Verify() {
		logger.Info("failed to verify transaction", zap.String("from", tx.From.String()),
			zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount))
		return false
	}

	//5、检查余额
	if tx.IsFreezeTransaction() {
		//检查to的可用余额
		balance, err := bc.GetBalance(tx.To.Bytes())
		if err != nil {
			logger.Error("failed to get balance", zap.Error(err), zap.String("to", tx.To.String()))
			return false
		}
		frozenBal, err := bc.GetFreezeBalance(tx.To.Bytes())
		if err != nil {
			logger.Error("failed to get freezebalance", zap.Error(err), zap.String("to", tx.To.String()))
		}
		if tx.Amount < MinAmount || util.Uint64SubOverflow(balance, frozenBal, tx.Amount) {
			//if tx.Amount < MinAmount || tx.Amount+frozenBal > balance {
			logger.Info("failed to verify amount", zap.String("to", tx.To.String()),
				zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount), zap.Uint64("avaliable balance", balance-frozenBal))
			return false
		}
	} else if tx.IsUnfreezeTransaction() {
		//检查to的已冻结余额
		frozenBal, err := bc.GetFreezeBalance(tx.To.Bytes())
		if err != nil {
			logger.Error("failed to get freezebalance", zap.Error(err), zap.String("to", tx.To.String()))
		}
		if tx.Amount < MinAmount || util.Uint64SubOverflow(frozenBal, tx.Amount) {
			//if tx.Amount < MinAmount || tx.Amount > frozenBal {
			logger.Info("failed to verify amount", zap.String("to", tx.To.String()),
				zap.String("to", tx.To.String()), zap.Uint64("amount", tx.Amount), zap.Uint64("frozen balance", frozenBal))
			return false
		}
	} else if tx.IsTransferTrasnaction() {
		//检查from的可用余额
		balance, err := bc.GetBalance(tx.From.Bytes())
		if err != nil {
			logger.Error("failed to get balance", zap.Error(err), zap.String("from", tx.From.String()))
			return false
		}
		frozenBal, err := bc.GetFreezeBalance(tx.From.Bytes())
		if err != nil {
			logger.Error("failed to get freezeBal", zap.Error(err), zap.String("from", tx.From.String()))
			return false
		}

		if tx.Amount < MinAmount || util.Uint64SubOverflow(balance, frozenBal, tx.Amount, tx.Fee) {
			logger.Info("failed to verify amount", zap.String("from", tx.From.String()), zap.String("to", tx.To.String()),
				zap.Uint64("amount", tx.Amount), zap.Uint64("unlockbalance", balance-frozenBal))
			return false
		}

		if tx.IsTokenTransaction() && (tx.Fee < MinAmount || util.Uint64SubOverflow(balance, frozenBal, tx.Amount, tx.Fee)) {
			return false
		}
	} else if !tx.IsCoinBaseTransaction() {
		logger.Error("wrong transaction type", zap.Uint16("tag", tx.Tag))
	}

	return true
}

// VerifyBlock 检查区块的默克尔根
func VerifyBlock(b block.Block, Bc blockchain.Blockchains) bool {

	trans := make([][]byte, 0, len(b.Transactions))
	for _, tx := range b.Transactions {
		trans = append(trans, tx.Serialize())
	}

	if trans != nil {
		tree := merkle.New(sha256.New(), trans)
		if ok := tree.VerifyNode(b.Root); ok {
			logger.Error("Faile to verify node")
			return false
		}

		for _, tx := range b.Transactions {
			if !verify(*tx, Bc) {
				logger.Error("Failed to verify transaction")
				return false
			}
		}
	}
	return true
}

// Filter 过滤掉不符合的交易
func (pool *TxPool) Filter(block block.Block) {
	txs := []*transaction.Transaction(*pool.List)
	txsLenght := len(txs)
	now := time.Now().UTC().Unix()
	for j := 0; j < txsLenght; j++ {
		for _, btx := range block.Transactions {
			//去除与块中重叠或存在超过10s的交易
			if txs[j].EqualNonce(btx) || now-txs[j].Time > 10 {
				txs = append(txs[:j], txs[j+1:]...)
				txsLenght--
				break
			}
		}
	}
	*pool.List = TxHeap(txs[:txsLenght])
}

// SetCheckData 设置检查的数据
func (pool *TxPool) SetCheckData(data []byte) error {
	pool.Mutex.Lock()
	defer pool.Mutex.Unlock()

	var cb CheckBlock
	err := json.Unmarshal(data, &cb)
	if err != nil {
		return err
	}
	key := cb.Nodeid + string(cb.Hash)
	if _, ok := pool.Idhc[key]; ok {
		logger.Info("The key exists.")
		return nil
	}
	pool.Idhc[key] = cb
	logger.Info("SetCheckData", zap.String("node id", cb.Nodeid), zap.Uint64("height", cb.Height), zap.Bool("code", cb.Code))
	return nil
}
