package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"kortho/block"
	"kortho/contract/exec"
	"kortho/contract/parser"
	"kortho/logger"
	"kortho/transaction"
	"kortho/types"
	"kortho/util"
	"kortho/util/merkle"
	"kortho/util/miscellaneous"
	"kortho/util/store"
	"kortho/util/store/bg"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"go.uber.org/zap"
)

const (
	// BlockchainDBName blockchain数据库名称
	BlockchainDBName = "blockchain.db"
	// ContractDBName 合约数据库名称
	ContractDBName = "contract.db"
)

const (
	MAXUINT64 = ^uint64(0)
)

var (
	// HeightKey 数据库中存储高度的键
	HeightKey = []byte("height")
	// NonceKey 存储nonce的map名
	NonceKey = []byte("nonce")
	//FreezeKey 冻结金额的map名
	FreezeKey = []byte("freeze")

	//BheightKey = []byte("bheight")

	dKtoPrefix    = "dk"
	pckPrefix     = "pck"
	PckTotalName  = "pcktotal"
	DKtoTotalName = "dktototal"
)

var (
	// AddrListPrefix 每个addreess在在数据库中都维护一个列表，AddrListPrefix是列表名的前缀
	AddrListPrefix = []byte("addr")
	// HeightPrefix 块高key的前缀
	HeightPrefix = []byte("blockheight")
	// TxListName 交易列表的名字
	TxListName = []byte("txlist")
)

// Blockchain 区块链数据结构
type Blockchain struct {
	mu  sync.RWMutex
	db  store.DB
	cdb store.DB
}

type TXindex struct {
	Height uint64
	Index  uint64
}

// New 创建区块链对象
func New() *Blockchain {
	bgs := bg.New("blockchain.db")
	bgc := bg.New("contract.db")
	bc := &Blockchain{db: bgs, cdb: bgc}

	return bc
}

// GetBlockchain 获取blockchain对象
func GetBlockchain() *Blockchain {
	return &Blockchain{db: bg.New(BlockchainDBName), cdb: bg.New(ContractDBName)}
}

// NewBlock 通过输入的交易，新建block，minaddr,Ds,Cm,QTJ分别是矿工，社区，技术和趣淘鲸的地址
func (bc *Blockchain) NewBlock(txs []*transaction.Transaction, minaddr, Ds, Cm, QTJ types.Address) (*block.Block, error) {
	logger.Info("start to new block")
	var height, prevHeight uint64
	var prevHash []byte
	prevHeight, err := bc.GetHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return nil, err
	}

	height = prevHeight + 1
	if height > 1 {
		prevHash, err = bc.GetHash(prevHeight)
		if err != nil {
			logger.Error("failed to get hash", zap.Error(err), zap.Uint64("previous height", prevHeight))
			return nil, err
		}
	} else {
		prevHash = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	}

	//出币分配
	txs = Distr(txs, minaddr, Ds, Cm, QTJ, height)

	/* 	//锁仓收益分红规则，社区分配收益到链上执行，每天执行一次
	   	//分配锁仓收益
	   	if height%24*60*60 == 0 {
	   		err := bc.ShareOutBouns(txs)
	   		if err != nil {
	   			logger.Error("failed from shareoutbouns to do :", zap.Error(err))
	   		}
	   	} */
	//生成默克尔根,如果没有交易的话，调用GetMtHash会painc
	txBytesList := make([][]byte, 0, len(txs))
	for _, tx := range txs {
		tx.BlockNumber = height
		txBytesList = append(txBytesList, tx.Serialize())
	}
	tree := merkle.New(sha256.New(), txBytesList)
	root := tree.GetMtHash()

	block := &block.Block{
		Height:       height,
		PrevHash:     prevHash,
		Transactions: txs,
		Root:         root,
		Version:      1,
		Timestamp:    time.Now().Unix(),
		Miner:        minaddr,
	}
	block.SetHash()
	logger.Info("end to new block")
	return block, nil
}

// AddBlock 向数据库添加新的block数据，minaddr矿工地址
func (bc *Blockchain) AddBlock(block *block.Block, minaddr []byte) error {
	logger.Info("Start to commit block...")
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()
	var err error
	var height, prevHeight uint64
	//拿出块高
	prevHeight, err = bc.getHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return err
	}

	height = prevHeight + 1
	if block.Height != height {
		return fmt.Errorf("height error:current height=%d,commit height=%d", prevHeight, block.Height)
	}

	//高度->哈希
	hash := block.Hash
	if err = DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		return err
	}

	//哈希-> 块
	if err = DBTransaction.Set(hash, block.Serialize()); err != nil {
		logger.Error("Failed to set block", zap.Error(err))
		return err
	}

	//重置块高
	DBTransaction.Del(HeightKey)
	DBTransaction.Set(HeightKey, miscellaneous.E64func(height))

	// 获取pck和dkto的总数
	pckTotal, err := getPckTotal(DBTransaction)
	if err != nil {
		logger.Error("failed to get total of pck")
		return err
	}

	dKtoTotal, err := getDKtoTotal(DBTransaction)
	if err != nil {
		logger.Error("failed to get total of dkto")
		return err
	}

	for index, tx := range block.Transactions {
		if tx.IsCoinBaseTransaction() {
			if err = setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setToAccount(DBTransaction, tx); err != nil {
				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}
		} else if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			nonce := tx.Nonce + 1
			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
			}

			var frozenBalBytes []byte
			frozenBal, _ := bc.getFreezeBalance(tx.To.Bytes())
			if tx.IsFreezeTransaction() {
				frozenBalBytes = miscellaneous.E64func(tx.Amount + frozenBal)
				/* 			//投票记录处理
				vote := NewVote(tx)
				SetVote(*vote, DBTransaction) */
			} else {
				frozenBalBytes = miscellaneous.E64func(frozenBal - tx.Amount)
			}
			if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
				logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}
		} else if tx.IsConvertPckTransaction() || tx.IsConvertKtoTransaction() {
			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			//更新nonce,block中txs必须是有序的
			nonce := tx.Nonce + 1
			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			if tx.IsConvertPckTransaction() {
				if err := setConvertPck(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
					logger.Error("Failed to set pck", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
					return err
				}

				if util.Uint64AddOverflow(pckTotal, tx.PckNum) && util.Uint64AddOverflow(dKtoTotal, tx.KtoNum) {
					logger.Error("faile to verify total")
					return errors.New("uint64 overflow")
				}
				pckTotal += tx.PckNum
				dKtoTotal += tx.KtoNum
			} else {
				if err := setConvertKto(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
					logger.Error("Failed to set dkto", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
					return err
				}
				if pckTotal < tx.PckNum && dKtoTotal < tx.KtoNum {
					logger.Error("faile to verify total")
					return errors.New("uint64 overflow")
				}
				pckTotal -= tx.PckNum
				dKtoTotal -= tx.KtoNum

			}
		} else {
			if tx.IsTokenTransaction() {
				sc := parser.Parser([]byte(tx.Script))
				e, err := exec.New(bc.cdb, sc, tx.From.String())
				if err != nil {
					logger.Error("Failed to new exec", zap.String("script", tx.Script),
						zap.String("from address", tx.From.String()))
					return err
				}

				if err = e.Flush(); err != nil {
					logger.Error("Failed to flush exec", zap.String("script", tx.Script),
						zap.String("from address", tx.From.String()))
					return err
				}

				if err = setMinerFee(DBTransaction, minaddr, tx.Fee); err != nil {
					logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
						zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
					return err
				}
			}

			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			//更新nonce,block中txs必须是有序的
			nonce := tx.Nonce + 1
			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			//更新余额
			if err := setAccount(DBTransaction, tx); err != nil {
				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}
		}

		// if err := setTxList(DBTransaction, tx); err != nil {
		// 	logger.Error("Failed to set block data", zap.String("from", tx.From.String()), zap.Uint64("nonce", tx.Nonce))
		// 	return err
		// }
	}

	/* 	//固定周期处理投票结果
	   	if height%(30*24*60*60) == 0 {
	   		//处理投票结果
	   		Voteresult(DBTransaction)
		   } */

	if err := setPckAndDktoToatal(DBTransaction, pckTotal, dKtoTotal); err != nil {
		logger.Error("failed to set total", zap.Error(err))
		return err
	}

	logger.Info("end to commit block")
	return DBTransaction.Commit()
}

// GetNonce 获取address的nonce
func (bc *Blockchain) GetNonce(address []byte) (uint64, error) {
	bc.mu.RLock()

	nonceBytes, err := bc.db.Mget(NonceKey, address)
	if err == store.NotExist {
		bc.mu.RUnlock()
		return 1, bc.setNonce(address, 1)
	} else if err != nil {
		return 0, err
	}
	bc.mu.RUnlock()

	return miscellaneous.D64func(nonceBytes)
}

// func (bc *Blockchain) getNonce(address []byte) (uint64, error) {
// 	nonceBytes, err := bc.db.Mget(NonceKey, address)
// 	if err == store.NotExist {
// 		return 1, bc.setNonce(address, 1)
// 	} else if err != nil {
// 		return 0, err
// 	}

// 	return miscellaneous.D64func(nonceBytes)
// }

func (bc *Blockchain) setNonce(address []byte, nonce uint64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	nonceBytes := miscellaneous.E64func(nonce)
	return bc.db.Mset(NonceKey, address, nonceBytes)
}

// GetBalance 获取address的余额
func (bc *Blockchain) GetBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.getBalance(address)
}

func (bc *Blockchain) getBalance(address []byte) (uint64, error) {
	balanceBytes, err := bc.db.Get(address)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return miscellaneous.D64func(balanceBytes)
}

// GetHeight 获取当前块高
func (bc *Blockchain) GetHeight() (height uint64, err error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getHeight()
}

func (bc *Blockchain) getHeight() (uint64, error) {
	heightBytes, err := bc.db.Get(HeightKey)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return miscellaneous.D64func(heightBytes)
}

// GetHash 获取块高对应的hash
func (bc *Blockchain) GetHash(height uint64) (hash []byte, err error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
}

// GetBlockByHash 获取hash对应的块数据
func (bc *Blockchain) GetBlockByHash(hash []byte) (*block.Block, error) { //
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	blockData, err := bc.db.Get(hash)
	if err != nil {
		return nil, err
	}
	return block.Deserialize(blockData)
}

// GetBlockByHeight 获取块高对应的块
func (bc *Blockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBlockByheight(height)
}

func (bc *Blockchain) getBlockByheight(height uint64) (*block.Block, error) {
	if height < 1 {
		return nil, errors.New("parameter error")
	}

	// 1、先获取到hash
	hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
	if err != nil {
		return nil, err
	}

	// 2、通过hash获取block
	blockData, err := bc.db.Get(hash)
	if err != nil {
		return nil, err
	}

	return block.Deserialize(blockData)
}

func (bc *Blockchain) GBbyHeight(height uint64) (*block.Block, error) {
	if height < 1 {
		return nil, errors.New("parameter error")
	}

	// 1、先获取到hash
	hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
	if err != nil {
		return nil, err
	}

	// 2、通过hash获取block
	blockData, err := bc.db.Get(hash)
	if err != nil {
		return nil, err
	}

	return block.Deserialize(blockData)
}

/*
// GetTransactions 获取从start到end的所有交易
func (bc *Blockchain) GetTransactions(start, end int64) ([]*transaction.Transaction, error) {
	//获取hash的交易
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	hashList, err := bc.db.Lrange(TxListName, start, end)
	if err != nil {
		logger.Error("failed to get txlist", zap.Error(err))
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0, len(hashList))
	for _, hash := range hashList {
		txBytes, err := bc.db.Get(hash)
		if err != nil {
			return nil, err
		}

		transaction := &transaction.Transaction{}
		if err := json.Unmarshal(txBytes, transaction); err != nil {
			logger.Error("Failed to unmarshal bytes", zap.Error(err))
			return nil, err
		}

		transactions = append(transactions, transaction)
	}

	return transactions, err
}
*/
// GetTransactions 获取从start到end的所有交易
func (bc *Blockchain) GetTransactions(start, end int64) ([]*transaction.Transaction, error) {
	//获取hash的交易
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	hashList, err := bc.db.Lrange(TxListName, start, end)
	if err != nil {
		logger.Error("failed to get txlist", zap.Error(err))
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0, len(hashList))
	for _, hash := range hashList {
		txBytes, err := bc.getTransactionByHash(hash)
		if err != nil {
			return nil, err
		}

		// transaction := &transaction.Transaction{}
		// if err := json.Unmarshal(txBytes, transaction); err != nil {
		// 	logger.Error("Failed to unmarshal bytes", zap.Error(err))
		// 	return nil, err
		// }

		transactions = append(transactions, txBytes)
	}

	return transactions, err
}

// GetTransactionByHash 获取交易哈希对应的交易
func (bc *Blockchain) GetTransactionByHash(hash []byte) (*transaction.Transaction, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getTransactionByHash(hash)
}

func (bc *Blockchain) getTransactionByHash(hash []byte) (*transaction.Transaction, error) {

	Hi, err := bc.db.Get(hash)
	if err != nil {
		logger.Error("failed to get hash", zap.Error(err))
		return nil, err
	}
	var txindex TXindex
	err = json.Unmarshal(Hi, &txindex)
	if err != nil {
		logger.Error("Failed to unmarshal bytes", zap.Error(err))
		return nil, err
	}
	// bh, err := miscellaneous.D64func(Hi.Height)
	// if err != nil {
	// 	logger.Error("failed to get hash", zap.Error(err))
	// 	return nil, err
	// }
	b, err := bc.getBlockByheight(txindex.Height)
	if err != nil {
		logger.Error("failed to getblock height", zap.Error(err), zap.Uint64("height", txindex.Height))
		return nil, err
	}

	//transaction := &transaction.Transaction{}
	tx := b.Transactions[txindex.Index]
	// err = json.Unmarshal(tx, transaction)
	// if err != nil {
	// 	return nil, err
	// }

	return tx, nil
}

// // GetTransactionByAddr 获取address从start到end的所有交易
// func (bc *Blockchain) GetTransactionByAddr(address []byte, start, end int64) ([]*transaction.Transaction, error) {
// 	bc.mu.RLock()
// 	defer bc.mu.RUnlock()

// 	txHashList, err := bc.db.Lrange(append(AddrListPrefix, address...), start, end)
// 	if err != nil {
// 		logger.Error("failed to get addrhashlist", zap.Error(err))
// 		return nil, err
// 	}

// 	transactions := make([]*transaction.Transaction, 0, len(txHashList))
// 	for _, hash := range txHashList {
// 		txBytes, err := bc.db.Get(hash)
// 		if err != nil {
// 			logger.Error("Failed to get transaction", zap.Error(err), zap.ByteString("hash", hash))
// 			return nil, err
// 		}
// 		var tx transaction.Transaction
// 		if err := json.Unmarshal(txBytes, &tx); err != nil {
// 			logger.Error("Failed to unmarshal bytes", zap.Error(err))
// 			return nil, err
// 		}
// 		transactions = append(transactions, &tx)
// 	}

// 	return transactions, nil
// }

// GetTransactionByAddr 获取address从start到end的所有交易
func (bc *Blockchain) GetTransactionByAddr(address []byte, start, end int64) ([]*transaction.Transaction, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	//Mkeys([]byte) ([][]byte, error)
	txHashList, err := bc.db.Mkeys(address)
	if err != nil {
		logger.Error("failed to get addrhashlist", zap.Error(err))
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0, len(txHashList))
	ltx := len(txHashList)
	if uint64(end) > uint64(ltx) {
		end = int64(ltx)
	}

	for i := start; i < end; i++ {
		txBytes, err := bc.getTransactionByHash(txHashList[i])
		if err != nil {
			logger.Error("Failed to get transaction", zap.Error(err), zap.ByteString("hash", txHashList[i]))
			return nil, err
		}

		transactions = append(transactions, txBytes)
	}

	return transactions, nil
}

// GetContractDB 获取忽而学数据库对象
func (bc *Blockchain) GetContractDB() store.DB {
	return bc.cdb
}

// GetMaxBlockHeight 获取块高
func (bc *Blockchain) GetMaxBlockHeight() (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	heightBytes, err := bc.db.Get(HeightKey)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return miscellaneous.D64func(heightBytes)
}

// GetTokenBalance 获取代币余额
func (bc *Blockchain) GetTokenBalance(address, symbol []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return exec.Balance(bc.cdb, string(symbol), string(address))
}

func setMinerFee(tx store.Transaction, to []byte, amount uint64) error {
	tobalance, err := tx.Get(to)
	if err == store.NotExist {
		tobalance = miscellaneous.E64func(0)
	} else if err != nil {
		return err
	}

	toBalance, _ := miscellaneous.D64func(tobalance)
	toBalanceBytes := miscellaneous.E64func(toBalance + amount)

	return setBalance(tx, to, toBalanceBytes)
}

// Distr 出币分配
func Distr(txs []*transaction.Transaction, minaddr, Ds, Cm, QTJ types.Address, height uint64) []*transaction.Transaction {
	//TODO:避免魔数存在
	var orderIndexList []int
	var total uint64 = 49460000000
	x := height / 31536000 //矿工奖励衰减周期

	for i := 0; uint64(i) < x; i++ {
		total = total * 8 / 10
	}
	each, mod := total/10, total%10

	for i, tx := range txs {
		if tx.IsOrderTransaction() && tx.Order.Vertify(QTJ) {
			orderIndexList = append(orderIndexList, i)
		}
	}

	if len(orderIndexList) != 0 {
		fAmonut, fMod := each/uint64(len(orderIndexList)), each%uint64(len(orderIndexList)) //10% 订单用户
		for _, orderIndex := range orderIndexList {
			txs = append(txs, transaction.NewCoinBaseTransaction(txs[orderIndex].Order.Address, fAmonut))
		}

		dsAmount := each + fMod //10% 电商
		txs = append(txs, transaction.NewCoinBaseTransaction(Ds, dsAmount))
	} else {
		dsAmount := each * 2 //20% 电商
		txs = append(txs, transaction.NewCoinBaseTransaction(Ds, dsAmount))
	}

	jsAmount := each*4 + mod //40% 技术
	txs = append(txs, transaction.NewCoinBaseTransaction(minaddr, jsAmount))

	sqAmount := each * 4 //40% 社区
	//	SetIncome(Samount)
	txs = append(txs, transaction.NewCoinBaseTransaction(Cm, sqAmount))

	return txs
}

func setTxbyaddr(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction) error {
	// txBytes, _ := json.Marshal(tx)
	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
	listNmae := append(AddrListPrefix, addr...)
	_, err := DBTransaction.Llpush(listNmae, tx.Hash)
	return err
}

func setTxbyaddrKV(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction, index uint64) error {
	// txBytes, _ := json.Marshal(tx)
	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
	DBTransaction.Mset(addr, tx.Hash, []byte(""))
	txindex := &TXindex{
		Height: tx.BlockNumber,
		Index:  index,
	}
	tdex, err := json.Marshal(txindex)
	if err != nil {
		logger.Error("Failed Marshal txindex", zap.Error(err))
		return err
	}
	DBTransaction.Set(tx.Hash, tdex)
	return err
}

func setNonce(DBTransaction store.Transaction, addr, nonce []byte) error {
	DBTransaction.Mdel(NonceKey, addr)
	return DBTransaction.Mset(NonceKey, addr, nonce)
}

func setTxList(DBTransaction store.Transaction, tx *transaction.Transaction) error {
	//TxList->txhash
	if _, err := DBTransaction.Llpush(TxListName, tx.Hash); err != nil {
		logger.Error("Failed to push txhash", zap.Error(err))
		return err
	}

	//交易hash->交易数据
	txBytes, _ := json.Marshal(tx)
	if err := DBTransaction.Set(tx.Hash, txBytes); err != nil {
		logger.Error("Failed to set transaction", zap.Error(err))
		return err
	}

	return nil
}

func setToAccount(dbTransaction store.Transaction, transaction *transaction.Transaction) error {
	var balance uint64
	balanceBytes, err := dbTransaction.Get(transaction.To.Bytes())
	if err != nil {
		balance = 0
	} else {
		balance, err = miscellaneous.D64func(balanceBytes)
		if err != nil {
			return err
		}
	}

	newBalanceBytes := miscellaneous.E64func(balance + transaction.Amount)
	if err := setBalance(dbTransaction, transaction.To.Bytes(), newBalanceBytes); err != nil {
		return err
	}

	return nil
}

func setAccount(DBTransaction store.Transaction, tx *transaction.Transaction) error {
	from, to := tx.From.Bytes(), tx.To.Bytes()

	fromBalBytes, _ := DBTransaction.Get(from)
	fromBalance, _ := miscellaneous.D64func(fromBalBytes)
	if tx.IsTokenTransaction() {
		fromBalance -= tx.Amount + tx.Fee
	} else {
		fromBalance -= tx.Amount
	}

	tobalance, err := DBTransaction.Get(to)
	if err != nil {
		setBalance(DBTransaction, to, miscellaneous.E64func(0))
		tobalance = miscellaneous.E64func(0)
	}

	toBalance, _ := miscellaneous.D64func(tobalance)
	toBalance += tx.Amount

	Frombytes := miscellaneous.E64func(fromBalance)
	Tobytes := miscellaneous.E64func(toBalance)

	if err := setBalance(DBTransaction, from, Frombytes); err != nil {
		return err
	}
	if err := setBalance(DBTransaction, to, Tobytes); err != nil {
		return err
	}

	return nil
}

func setBalance(tx store.Transaction, addr, balance []byte) error {
	tx.Del(addr)
	return tx.Set(addr, balance)
}

func setFreezeBalance(tx store.Transaction, addr, freezeBal []byte) error {
	return tx.Mset(FreezeKey, addr, freezeBal)
}

// GetFreezeBalance 获取被冻结的金额
func (bc *Blockchain) GetFreezeBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getFreezeBalance(address)
}

func (bc *Blockchain) getFreezeBalance(address []byte) (uint64, error) {
	freezeBalBytes, err := bc.db.Mget(FreezeKey, address)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return miscellaneous.D64func(freezeBalBytes)
}

// CalculationResults 计算出该block上链后个地址的可用余额，如果余额不正确则返回错误
func (bc *Blockchain) CalculationResults(block *block.Block) ([]byte, error) {
	//TODO:计算出余额后，进行hash
	var ok bool
	var err error
	var avlBalance, frozenBalance uint64
	var pckDkto struct {
		pck  uint64
		dkto uint64
	}
	//block.Results = make(map[string]uint64)
	avlBalanceResults := make(map[string]uint64)
	frozenBalanceResults := make(map[string]uint64)
	pckDktoResults := make(map[string]struct {
		pck  uint64
		dkto uint64
	})
	for _, tx := range block.Transactions {
		//1、from余额计算
		if tx.IsTransferTrasnaction() || tx.IsConvertKtoTransaction() || tx.IsConvertPckTransaction() {
			if avlBalance, ok = avlBalanceResults[tx.From.String()]; !ok {
				balance, err := bc.GetBalance(tx.From.Bytes())
				if err != nil {
					return nil, err
				}

				frozenBalance, err := bc.GetFreezeBalance(tx.From.Bytes())
				if err != nil {
					return nil, err
				}

				if util.Uint64SubOverflow(balance, frozenBalance) {
					logger.Info("sub overflow", zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
					return nil, errors.New("insufficient balance")
				}

				avlBalance = balance - frozenBalance
			}

			if tx.IsConvertKtoTransaction() || tx.IsConvertPckTransaction() {
				if pckDkto, ok = pckDktoResults[tx.From.String()]; !ok {
					pckDkto.dkto, err = bc.GetDKto(tx.From.Bytes())
					if err != nil {
						return nil, err
					}

					pckDkto.pck, err = bc.GetPck(tx.From.Bytes())
					if err != nil {
						return nil, err
					}
				}
			}

			if tx.IsTransferTrasnaction() {
				if util.Uint64SubOverflow(avlBalance, tx.Amount, tx.Fee) {
					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("amount", tx.Amount),
						zap.Uint64("fee", tx.Fee))
					return nil, errors.New("insufficient balance")
				}
				avlBalanceResults[tx.From.String()] = avlBalance - tx.Amount - tx.Fee
			} else if tx.IsConvertPckTransaction() {
				if avlBalance < tx.KtoNum || util.Uint64AddOverflow(pckDkto.pck, tx.PckNum) ||
					util.Uint64AddOverflow(pckDkto.dkto, tx.KtoNum) {
					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("ktonum", tx.KtoNum),
						zap.Uint64("pck balance", pckDkto.pck), zap.Uint64("pck number", tx.PckNum), zap.Uint64("dkto balance", pckDkto.dkto))
					return nil, errors.New("insufficient balance")
				}
				avlBalanceResults[tx.From.String()] = avlBalance - tx.KtoNum
				pckDktoResults[tx.From.String()] = struct {
					pck  uint64
					dkto uint64
				}{pckDkto.pck + tx.PckNum, pckDkto.dkto + tx.KtoNum}
			} else if tx.IsConvertKtoTransaction() {
				if pckDkto.pck < tx.PckNum || pckDkto.dkto < tx.KtoNum || util.Uint64AddOverflow(avlBalance, tx.KtoNum) {
					logger.Info("add overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("ktonum", tx.KtoNum),
						zap.Uint64("pck balance", pckDkto.pck), zap.Uint64("pck number", tx.PckNum), zap.Uint64("dkto balance", pckDkto.dkto))
					return nil, errors.New("insufficient balance")
				}
				avlBalanceResults[tx.From.String()] = avlBalance + tx.KtoNum
				pckDktoResults[tx.From.String()] = struct {
					pck  uint64
					dkto uint64
				}{pckDkto.pck - tx.PckNum, pckDkto.dkto - tx.KtoNum}
			}
		}

		//2、to余额计算
		if !tx.IsConvertKtoTransaction() && !tx.IsConvertPckTransaction() {
			if avlBalance, ok = avlBalanceResults[tx.To.String()]; !ok {
				balance, err := bc.GetBalance(tx.To.Bytes())
				if err != nil {
					return nil, err
				}

				frozenBalance, err := bc.GetFreezeBalance(tx.To.Bytes())
				if err != nil {
					return nil, err
				}

				if util.Uint64SubOverflow(balance, frozenBalance) {
					logger.Info("sub overflow", zap.String("address", tx.To.String()),
						zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
					return nil, errors.New("insufficient balance")
				}
				logger.Info("Balance information", zap.String("address", tx.To.String()),
					zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
				avlBalance = balance - frozenBalance
			}

			if frozenBalance, ok = frozenBalanceResults[tx.To.String()]; !ok {
				frozenBalance, err = bc.getFreezeBalance(tx.To.Bytes())
				if err != nil {
					return nil, err
				}
			}

			if tx.IsCoinBaseTransaction() || tx.IsTransferTrasnaction() {
				avlBalanceResults[tx.To.String()] = avlBalance + tx.Amount
			} else if tx.IsFreezeTransaction() {
				//TODO:处理冻结金额大于余额的情况
				if avlBalance < tx.Amount {
					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("amount", tx.Amount))
					return nil, errors.New("insufficient balance")
				}
				avlBalanceResults[tx.To.String()] = avlBalance - tx.Amount
				frozenBalanceResults[tx.To.String()] = frozenBalance + tx.Amount
			} else if tx.IsUnfreezeTransaction() {
				if frozenBalance < tx.Amount {
					logger.Info("sub overflow", zap.Uint64("frozen balance", frozenBalance), zap.Uint64("amount", tx.Amount))
					return nil, errors.New("insufficient frozen balance")
				}
				frozenBalanceResults[tx.To.String()] = frozenBalance - tx.Amount
				avlBalanceResults[tx.To.String()] = avlBalance + tx.Amount
			} else {
				return nil, errors.New("wrong transaction type")
			}
		}

	}

	var buf bytes.Buffer
	var avlKeys, frzKeys, pckDktoKeys []string

	for key := range avlBalanceResults {
		avlKeys = append(avlKeys, key)
	}
	sort.Strings(avlKeys)

	for key := range frozenBalanceResults {
		frzKeys = append(frzKeys, key)
	}
	sort.Strings(frzKeys)

	for key := range pckDktoResults {
		pckDktoKeys = append(pckDktoKeys, key)
	}
	sort.Strings(pckDktoKeys)

	for _, key := range avlKeys {
		value := avlBalanceResults[key]
		addr, _ := types.StringToAddress(key)
		valBytes := miscellaneous.E64func(value)
		buf.Write(addr.Bytes())
		buf.Write(valBytes)
	}

	for _, key := range frzKeys {
		value := frozenBalanceResults[key]
		addr, _ := types.StringToAddress(key)
		valBytes := miscellaneous.E64func(value)
		buf.Write(addr.Bytes())
		buf.Write(valBytes)
	}

	for _, key := range pckDktoKeys {
		value := pckDktoResults[key]
		addr, _ := types.StringToAddress(key)
		pckBytes := miscellaneous.E64func(value.pck)
		dktoBytes := miscellaneous.E64func(value.dkto)
		buf.Write(addr.Bytes())
		buf.Write(pckBytes)
		buf.Write(dktoBytes)
	}

	//TODO：把block中的results删除，换成hash
	hash := sha256.Sum256(buf.Bytes())

	return hash[:], nil
}

// CheckResults  重新计算结果，并与结果集对比，相同为true，否则为false
func (bc *Blockchain) CheckResults(block *block.Block, resultHash, Ds, Cm, qtj []byte) bool {
	//1、最后一笔交易必须是coinbase交易
	if !block.Transactions[len(block.Transactions)-1].IsCoinBaseTransaction() {
		logger.Error("the end is not a coinbase transaction")
		return false
	}

	//2、验证leader和follower的结果集是否相同
	currResultHash, err := bc.CalculationResults(block)
	if err != nil {
		logger.Error("failed to calculation results")
		return false
	}

	// for _, tx := range block.Transactions {
	// 	if tx.IsTokenTransaction() {
	// 		script := parser.Parser([]byte(tx.Script))
	// 		e, _ := exec.New(bc.cdb, script, string(tx.From.Bytes()))
	// 		ert := e.Root()
	// 		if hex.EncodeToString(tx.Root) != hex.EncodeToString(ert) {
	// 			logger.Error("scrpit", zap.String("root", hex.EncodeToString(tx.Root)), zap.String("ert", hex.EncodeToString(ert)))
	// 			return false
	// 		}
	// 	}

	// 	if !tx.IsCoinBaseTransaction() && !tx.IsFreezeTransaction() {
	// 		if balance, ok := block.Results[tx.From.String()]; !ok {
	// 			logger.Info("address is not exist", zap.String("from", tx.From.String()))
	// 			return false
	// 		} else if balance != currBlock.Results[tx.From.String()] {
	// 			logger.Info("balance is not equal", zap.Uint64("curBalnce", balance), zap.Uint64("resBalance", currBlock.Results[tx.From.String()]))
	// 			return false
	// 		}
	// 	}

	// 	if !tx.IsFreezeTransaction() {
	// 		if balance, ok := block.Results[tx.To.String()]; !ok {
	// 			logger.Info("address is not exist", zap.String("to", tx.To.String()))
	// 			return false
	// 		} else if balance != currBlock.Results[tx.To.String()] {
	// 			logger.Info("balance is not equal", zap.Uint64("curBalnce", balance), zap.Uint64("resBalance", currBlock.Results[tx.To.String()]))
	// 			return false
	// 		}
	// 	}
	// }
	//3、检查各个地址余额
	log.Debug("length", zap.Int("prev len", len(resultHash)), zap.Int("curr len", len(currResultHash)))
	if bytes.Compare(resultHash, currResultHash) != 0 {
		logger.Error("hash not equal")
		return false
	}

	return true
}

// GetBlockSection 获取冲lowH到heiH的所有block
func (bc *Blockchain) GetBlockSection(lowH, heiH uint64) ([]*block.Block, error) {
	var blocks []*block.Block
	for i := lowH; i <= heiH; i++ {
		hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(i)...))
		//hash, err := bc.db.Get(miscellaneous.E64func(i), HashKey)
		if err != nil {
			logger.Error("Failed to get hash", zap.Error(err), zap.Uint64("height", i))
			return nil, err
		}
		B, err := bc.db.Get(hash)
		if err != nil {
			logger.Error("Failed to get block", zap.Error(err), zap.String("hash", string(hash)))
			return nil, err
		}

		blcok := &block.Block{}
		if err := json.Unmarshal(B, blcok); err != nil {
			logger.Error("Failed to unmarshal block", zap.Error(err), zap.String("hash", string(hash)))
			return nil, err
		}
		blocks = append(blocks, blcok)
	}
	return blocks, nil
}

// GetTokenRoot 获取token的根
func (bc *Blockchain) GetTokenRoot(address, script string) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	e, err := exec.New(bc.GetContractDB(), parser.Parser([]byte(script)), address)
	if err != nil {
		return nil, err
	}

	return e.Root(), nil
}

//DeleteBlock delete block
func (bc *Blockchain) DeleteBlock(height uint64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()

	dbHeight, err := bc.getHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return err
	}

	if height > dbHeight {
		return fmt.Errorf("Wrong height to delete,[%v] should <= current height[%v]", height, dbHeight)
	}

	for dH := dbHeight; dH >= height; dH-- {
		logger.Info("Start to delete block", zap.Uint64("height", dH))
		block, err := bc.getBlockByheight(dH)
		if err != nil {
			logger.Error("failed to get block", zap.Error(err))
			return err
		}

		for i, tx := range block.Transactions {
			if tx.IsCoinBaseTransaction() {
				if err = deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.Uint64("amount", tx.Amount))
					return err
				}

				if err := delToAccount(DBTransaction, tx); err != nil {
					logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.Uint64("amount", tx.Amount))
					return err
				}
			} else if !tx.IsTransferTrasnaction() {
				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				nonce := tx.Nonce
				if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
					logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				}

				var frozenBalBytes []byte
				frozenBal, _ := bc.getFreezeBalance(tx.To.Bytes())
				if tx.IsFreezeTransaction() {
					frozenBalBytes = miscellaneous.E64func(tx.Amount - frozenBal)
				} else {
					frozenBalBytes = miscellaneous.E64func(frozenBal + tx.Amount)
				}
				if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
					logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
						zap.Uint64("amount", tx.Amount))
					return err
				}
			} else {
				if tx.IsTokenTransaction() {
					spilt := strings.Split(tx.Script, "\"")
					if spilt[0] == "transfer " {
						script := fmt.Sprintf("transfer \"%s\" %s \"%s\"", spilt[1], spilt[2], tx.From.String())

						sc := parser.Parser([]byte(script))
						e, err := exec.New(bc.cdb, sc, tx.To.String())
						if err != nil {
							logger.Error("Failed to new exec", zap.String("script", script),
								zap.String("from address", tx.To.String()))
							return err

						}

						if err = e.Flush(); err != nil {
							logger.Error("Failed to flush exec", zap.String("script", script),
								zap.String("from address", tx.To.String()))
							return err
						}
					}
					if err = delMinerFee(DBTransaction, block.Miner.Bytes(), tx.Fee); err != nil {
						logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
							zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
						return err
					}
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				//更新nonce,block中txs必须是有序的
				nonce := tx.Nonce
				if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
					logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				//更新余额
				if err := setAccount(DBTransaction, tx); err != nil {
					logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}
			}

			// if err := setTxList(DBTransaction, tx); err != nil {
			// 	logger.Error("Failed to set block data", zap.String("from", tx.From.String()), zap.Uint64("nonce", tx.Nonce))
			// 	return err
			// }
		}

		//高度->哈希
		hash := block.Hash
		if err = DBTransaction.Del(append(HeightPrefix, miscellaneous.E64func(block.Height)...)); err != nil {
			logger.Error("Failed to Del height and hash", zap.Error(err))
			return err
		}

		//哈希-> 块
		if err = DBTransaction.Del(hash); err != nil {
			logger.Error("Failed to Del block", zap.Error(err))
			return err
		}
		// last
		DBTransaction.Set(HeightKey, miscellaneous.E64func(dH-1))
	}

	logger.Info("End delete")
	return DBTransaction.Commit()
}

func deleteTxbyaddrKV(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction, index uint64) error {
	// txBytes, _ := json.Marshal(tx)
	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
	DBTransaction.Mdel(addr, tx.Hash)
	// txindex := &TXindex{
	// 	Height: tx.BlockNumber,
	// 	Index:  index,
	// }
	// tdex, err := json.Marshal(txindex)
	// if err != nil {
	// 	logger.Error("Failed Marshal txindex", zap.Error(err))
	// 	return err
	// }
	err := DBTransaction.Del(tx.Hash)
	if err != nil {
		logger.Error("Failed Marshal txindex", zap.Error(err))
		return err
	}
	return err
}

func delToAccount(dbTransaction store.Transaction, transaction *transaction.Transaction) error {
	var balance uint64
	balanceBytes, err := dbTransaction.Get(transaction.To.Bytes())
	if err != nil {
		balance = 0
	} else {
		balance, err = miscellaneous.D64func(balanceBytes)
		if err != nil {
			return err
		}
	}

	newBalanceBytes := miscellaneous.E64func(balance - transaction.Amount)
	if err := setBalance(dbTransaction, transaction.To.Bytes(), newBalanceBytes); err != nil {
		return err
	}

	return nil
}

func delAccount(DBTransaction store.Transaction, tx *transaction.Transaction) error {
	from, to := tx.From.Bytes(), tx.To.Bytes()

	fromBalBytes, _ := DBTransaction.Get(from)
	fromBalance, _ := miscellaneous.D64func(fromBalBytes)
	if tx.IsTokenTransaction() {
		fromBalance = fromBalance + tx.Amount + tx.Fee
	} else {
		fromBalance += tx.Amount
	}

	tobalance, err := DBTransaction.Get(to)
	if err != nil {
		setBalance(DBTransaction, to, miscellaneous.E64func(0))
		tobalance = miscellaneous.E64func(0)
	}

	toBalance, _ := miscellaneous.D64func(tobalance)
	toBalance -= tx.Amount

	Frombytes := miscellaneous.E64func(fromBalance)
	Tobytes := miscellaneous.E64func(toBalance)

	if err := setBalance(DBTransaction, from, Frombytes); err != nil {
		return err
	}
	if err := setBalance(DBTransaction, to, Tobytes); err != nil {
		return err
	}

	return nil
}

func delMinerFee(tx store.Transaction, to []byte, amount uint64) error {
	tobalance, err := tx.Get(to)
	if err == store.NotExist {
		tobalance = miscellaneous.E64func(0)
	} else if err != nil {
		return err
	}

	toBalance, _ := miscellaneous.D64func(tobalance)
	toBalanceBytes := miscellaneous.E64func(toBalance - amount)

	return setBalance(tx, to, toBalanceBytes)
}

//RecoverBlock 向数据库添加新的block数据，minaddr矿工地址
func (bc *Blockchain) RecoverBlock(block *block.Block, minaddr []byte) error {
	logger.Info("Start to recover block...", zap.Uint64("height", block.Height))
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()
	var err error
	var height, prevHeight uint64
	//拿出块高
	prevHeight, err = bc.getHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return err
	}

	height = prevHeight + 1
	if block.Height != height {
		return fmt.Errorf("height error:previous height=%d,current height=%d", prevHeight, height)
	}

	//高度->哈希
	hash := block.Hash
	if err = DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		return err
	}

	//哈希-> 块
	if err = DBTransaction.Set(hash, block.Serialize()); err != nil {
		logger.Error("Failed to set block", zap.Error(err))
		return err
	}

	//重置块高
	DBTransaction.Del(HeightKey)
	DBTransaction.Set(HeightKey, miscellaneous.E64func(height))

	for index, tx := range block.Transactions {
		if tx.IsCoinBaseTransaction() {
			if err = setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setToAccount(DBTransaction, tx); err != nil {
				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}
		} else if !tx.IsTransferTrasnaction() {
			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			nonce := tx.Nonce + 1
			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
			}

			var frozenBalBytes []byte
			frozenBal, _ := bc.getFreezeBalance(tx.To.Bytes())
			if tx.IsFreezeTransaction() {
				frozenBalBytes = miscellaneous.E64func(tx.Amount + frozenBal)
			} else {
				frozenBalBytes = miscellaneous.E64func(frozenBal - tx.Amount)
			}
			if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
				logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
					zap.Uint64("amount", tx.Amount))
				return err
			}
		} else {
			if tx.IsTokenTransaction() {
				spilt := strings.Split(tx.Script, "\"")
				if spilt[0] == "transfer " {
					sc := parser.Parser([]byte(tx.Script))
					e, err := exec.New(bc.cdb, sc, tx.From.String())
					if err != nil {
						logger.Error("Failed to new exec", zap.String("script", tx.Script),
							zap.String("from address", tx.From.String()))
						return err
					}

					if err = e.Flush(); err != nil {
						logger.Error("Failed to flush exec", zap.String("script", tx.Script),
							zap.String("from address", tx.From.String()))
						return err
					}

				}

				if err = setMinerFee(DBTransaction, minaddr, tx.Fee); err != nil {
					logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
						zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
					return err
				}
			}

			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			//更新nonce,block中txs必须是有序的
			nonce := tx.Nonce + 1
			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}

			//更新余额
			if err := setAccount(DBTransaction, tx); err != nil {
				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
				return err
			}
		}

		if err := setTxList(DBTransaction, tx); err != nil {
			logger.Error("Failed to set block data", zap.String("from", tx.From.String()), zap.Uint64("nonce", tx.Nonce))
			return err
		}
	}
	logger.Info("End recover.")
	return DBTransaction.Commit()
}

func setConvertPck(tx store.Transaction, from []byte, ktoNum, pckNum uint64) error {
	var bal, pckBal, dKto uint64
	// pck
	pckKey := append([]byte(pckPrefix), from...)
	pckBalBytes, err := tx.Get(pckKey)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		pckBal = 0
	} else {
		pckBal, _ = miscellaneous.D64func(pckBalBytes)
	}

	if MAXUINT64-pckBal < pckNum {
		return errors.New("integer overflow")
	}

	if err := tx.Set(pckKey, miscellaneous.E64func(pckBal+pckNum)); err != nil {
		return err
	}

	// dKto
	dKtoKey := append([]byte(dKtoPrefix), from...)
	dKtoBytes, err := tx.Get(dKtoKey)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		dKto = 0
	} else {
		dKto, _ = miscellaneous.D64func(dKtoBytes)
	}

	if MAXUINT64-dKto < ktoNum {
		return errors.New("integer overflow")
	}

	if err := tx.Set(dKtoKey, miscellaneous.E64func(dKto+ktoNum)); err != nil {
		return err
	}

	// 余额
	balBytes, err := tx.Get(from)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		bal = 0
	} else {
		bal, _ = miscellaneous.D64func(balBytes)
	}

	if bal < ktoNum {
		fmt.Println(bal)
		return errors.New("integer overflow")
	}

	if err := tx.Set(from, miscellaneous.E64func(bal-ktoNum)); err != nil {
		return err
	}

	return nil
}

func setConvertKto(tx store.Transaction, from []byte, ktoNum, pckNum uint64) error {
	var bal, pckBal, dKto uint64
	// pck
	pckKey := append([]byte(pckPrefix), from...)
	pckBalBytes, err := tx.Get(pckKey)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		pckBal = 0
	} else {
		pckBal, _ = miscellaneous.D64func(pckBalBytes)
	}

	if pckBal < pckNum {
		return errors.New("integer overflow")
	}

	if err := tx.Set(pckKey, miscellaneous.E64func(pckBal-pckNum)); err != nil {
		return err
	}

	// dKto
	dKtoKey := append([]byte(dKtoPrefix), from...)
	dKtoBytes, err := tx.Get(dKtoKey)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		dKto = 0
	} else {
		dKto, _ = miscellaneous.D64func(dKtoBytes)
	}

	if dKto < ktoNum {
		return errors.New("integer overflow")
	}

	if err := tx.Set(dKtoKey, miscellaneous.E64func(dKto-ktoNum)); err != nil {
		return err
	}

	// 余额
	balBytes, err := tx.Get(from)
	if err != nil && err != store.NotExist {
		return err
	}

	if err == store.NotExist {
		bal = 0
	} else {
		bal, _ = miscellaneous.D64func(balBytes)
	}

	if MAXUINT64-bal < ktoNum {
		return errors.New("integer overflow")
	}

	if err := tx.Set(from, miscellaneous.E64func(bal+ktoNum)); err != nil {
		return err
	}

	return nil
}

func getPckTotal(tx store.Transaction) (uint64, error) {
	data, err := tx.Get([]byte(PckTotalName))
	if err != nil && err != store.NotExist {
		return 0, err
	}

	if err == store.NotExist {
		return 0, nil
	}

	return miscellaneous.D64func(data)
}

func getDKtoTotal(tx store.Transaction) (uint64, error) {
	data, err := tx.Get([]byte(DKtoTotalName))
	if err != nil && err != store.NotExist {
		return 0, err
	}

	if err == store.NotExist {
		return 0, nil
	}

	return miscellaneous.D64func(data)
}

func (Bc *Blockchain) GetPckTotal() (uint64, error) {
	tx := Bc.db.NewTransaction()
	defer tx.Cancel()

	total, err := getPckTotal(tx)
	if err != nil {
		return 0, err
	}
	return total, tx.Commit()
}

func (Bc *Blockchain) GetDKtoTotal() (uint64, error) {
	tx := Bc.db.NewTransaction()
	defer tx.Cancel()
	total, err := getDKtoTotal(tx)
	if err != nil {
		return 0, err
	}
	return total, tx.Commit()
}

func setPckAndDktoToatal(tx store.Transaction, pckTotal, dKtoTotal uint64) error {
	if err := tx.Set([]byte(PckTotalName), miscellaneous.E64func(pckTotal)); err != nil {
		return err
	}
	if err := tx.Set([]byte(DKtoTotalName), miscellaneous.E64func(dKtoTotal)); err != nil {
		return err
	}
	return nil
}

func getPck(tx store.Transaction, addr []byte) (uint64, error) {
	var num uint64
	key := append([]byte(pckPrefix), addr...)
	data, err := tx.Get(key)
	if err != nil && err != store.NotExist {
		return 0, err
	}
	if err == store.NotExist {
		num = 0
	} else {
		num, _ = miscellaneous.D64func(data)
	}
	return num, nil
}

func getDKto(tx store.Transaction, addr []byte) (uint64, error) {
	var num uint64
	key := append([]byte(dKtoPrefix), addr...)

	data, err := tx.Get(key)
	if err != nil && err != store.NotExist {
		return 0, err
	}
	if err == store.NotExist {
		num = 0
	} else {
		num, _ = miscellaneous.D64func(data)
	}
	return num, nil
}

func (Bc *Blockchain) GetPck(addr []byte) (uint64, error) {
	tx := Bc.db.NewTransaction()
	defer tx.Cancel()
	num, err := getPck(tx, addr)
	if err != nil {
		return 0, err
	}
	return num, tx.Commit()
}

func (Bc *Blockchain) GetDKto(addr []byte) (uint64, error) {
	tx := Bc.db.NewTransaction()
	defer tx.Cancel()

	num, err := getDKto(tx, addr)
	if err != nil {
		return 0, err
	}

	return num, tx.Commit()
}

func (Bc *Blockchain) GetTokenDemic(symbol []byte) (uint64, error) {

	Bc.mu.RLock()
	defer Bc.mu.RUnlock()

	b, err := exec.Precision(Bc.cdb, string(symbol)) // 2，代比， 3，地址
	if err != nil {
		return 0, err
	}
	return b, nil
}
