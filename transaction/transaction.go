package transaction

import (
	"bytes"
	"encoding/json"
	"kortho/types"
	"kortho/util/miscellaneous"
	"time"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/sha3"
)

const (
	// AddrLength 地址长度
	AddrLength = 44
)

const (
	// TransferTag 转账标记
	TransferTag uint16 = iota
	// MinerTag 矿工标记
	MinerTag
	// FreezeTag 锁仓标记
	FreezeTag
	// UnfreezeTag 解锁标记
	UnfreezeTag
)

// AdminAddr 用来锁仓的管理员地址
var AdminAddr string

// Transaction 交易信息
type Transaction struct {
	//Nonce 自增的正整数，同一地址当前交易必定比上次大一
	Nonce uint64 `json:"nonce"`

	// BlockNumber 当前交易所在块的块高
	BlockNumber uint64 `json:"blocknumber"`

	// Amount 交易的金额
	Amount uint64 `json:"amount"`

	// From 交易的发起方地址
	From types.Address `json:"from"`

	// To 交易的接收方地址
	To types.Address `json:"to"`

	// Hash 交易hash
	Hash []byte `json:"hash"`

	// Signature 交易的签名
	Signature []byte `json:"signature"`

	// Time 发起交易的时间时间戳，以秒为单位
	Time int64 `json:"time"`

	// Root 交易的默克尔根，用来进行交易数据的快速对比
	Root []byte `json:"root"`

	// Sctipt 代币的名称，非代币交易该字符串长度为0
	Script string `json:"script"`

	// Fee 代币交易的手续费，如果不是代币交易，此项为0
	Fee uint64 `json:"fee"`

	// Tag 用不同的数值，标记不同的交易类型
	//	0：转账交易
	//  1：矿工交易
	//	2：锁仓交易
	//	3：解锁交易
	Tag uint16 `json:"tag"`

	// Order 交易中携带的订单数据，没有订单此项为nil
	Order *Order `json:"ord"`
}

// Option 创建交易时的可选参数
type Option struct {
	Fee     uint64
	Script  string
	Root    []byte
	Message string
	Tag     uint16
	Ord     *Order
}

// ModOption 创建交易时可选参数的类型
type ModOption func(option *Option)

// WithToken 添加此参数，既是代币交易
func WithToken(fee uint64, script string, root []byte) ModOption {
	return func(option *Option) {
		option.Fee = fee
		option.Script = script
		option.Root = root
	}
}

// WithOrder 添加订单信息
func WithOrder(Order *Order) ModOption {
	return func(option *Option) {
		option.Ord = Order
	}
}

// func WithMessage(msg string) ModOption {
// 	return func(option *Option) {
// 		option.Message = msg
// 	}
// }

// WithFreezeBalance 添加锁仓交易标记
func WithFreezeBalance() ModOption {
	return func(option *Option) {
		option.Tag = FreezeTag
	}
}

// WithUnfreezeBalance 添加解锁交易标记
func WithUnfreezeBalance() ModOption {
	return func(option *Option) {
		option.Tag = UnfreezeTag
	}
}

func InitAdmin(address string) {
	AdminAddr = address
}

// ZNewTransaction 新建一个交易，其中nonce，amount，from，to是必须的参数
func ZNewTransaction(nonce, amount uint64, from, to types.Address, modOptions ...ModOption) *Transaction {
	var option Option
	for _, modOption := range modOptions {
		modOption(&option)
	}

	tx := &Transaction{
		Nonce:  nonce,
		Amount: amount,
		From:   from,
		To:     to,
		Time:   time.Now().Unix(),
		//Script、Fee、Root是代币的参数
		Fee:    option.Fee,
		Script: option.Script,
		Root:   option.Root,
		//Message: option.Message,
		Tag:   option.Tag,
		Order: option.Ord, //Ord是商城订单

	}
	tx.HashTransaction()

	return tx
}

// func NewTransaction(nonce, amount uint64, from, to types.Address, script string) *Transaction {
// 	tx := &Transaction{
// 		Nonce:  nonce,
// 		Amount: amount,
// 		From:   from,
// 		To:     to,
// 		Time:   time.Now().Unix(),
// 		Script: script,
// 	}

// 	tx.HashTransaction()
// 	return tx
// }

// NewCoinBaseTransaction 新建一个矿工交易
func NewCoinBaseTransaction(address types.Address, amount uint64) *Transaction {
	from := new(types.Address)
	transaction := Transaction{
		From:   *from,
		To:     address,
		Nonce:  0,
		Amount: amount,
		Time:   time.Now().Unix(),
		Tag:    MinerTag,
	}
	transaction.HashTransaction()
	return &transaction
}

// NewFreezeTransaction 新建锁仓交易
func NewFreezeTransaction(address types.Address, amount uint64, nonce uint64) *Transaction {
	to := new(types.Address)
	transaction := Transaction{
		From:  address,
		To:    *to,
		Nonce: nonce,
		Time:  time.Now().Unix(),
	}
	transaction.HashTransaction()
	return &transaction

}

// IsCoinBaseTransaction 如果是矿工交易返回true，否则返回false
func (tx *Transaction) IsCoinBaseTransaction() bool {
	return tx.From.IsNil() && tx.Tag == MinerTag
}

// IsTransferTrasnaction 如果是转账交易返回true，否则返回false
func (tx *Transaction) IsTransferTrasnaction() bool {
	return tx.Tag == TransferTag
}

// IsFreezeTransaction 如果是锁仓交易返回true,否则返回false
func (tx *Transaction) IsFreezeTransaction() bool {
	return bytes.Equal(tx.From.Bytes(), []byte(AdminAddr)) && tx.Tag == FreezeTag
}

// IsUnfreezeTransaction 如果该交易是解锁交易返回ture，否则返回false
func (tx *Transaction) IsUnfreezeTransaction() bool {
	return bytes.Equal(tx.From.Bytes(), []byte(AdminAddr)) && tx.Tag == UnfreezeTag
}

// IsTokenTransaction 如果是代币交易返回ture，否则返回false
func (tx *Transaction) IsTokenTransaction() bool {
	if len(tx.Script) != 0 && tx.Fee != 0 {
		return true
	}
	return false
}

// IsOrderTransaction 如果交易中含有订单信息返回ture，否则返回false
func (tx *Transaction) IsOrderTransaction() bool {
	if tx.Order != nil && tx.Order.Signature != nil && len(tx.Order.Signature) != 0 {
		return true
	}
	return false
}

// Serialize 序列化交易信息
func (tx *Transaction) Serialize() []byte {
	txBytes, _ := json.Marshal(tx)
	return txBytes
}

// Deserialize 反序列化json形式的交易数据，成功返回交易对象指针，否则会返回error
func Deserialize(data []byte) (*Transaction, error) {
	var tx Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// GetTime 获取交易的时间戳，以秒为单位
func (tx *Transaction) GetTime() int64 {
	return tx.Time
}

// GetNonce 获取交易的随机数
func (tx *Transaction) GetNonce() int64 {
	return int64(tx.Nonce)
}

// func Newtoken(nonce, amount, fee uint64, from, to types.Address, script string) *Transaction {
// 	tx := &Transaction{
// 		Nonce:  nonce,
// 		Amount: amount,
// 		From:   from,
// 		To:     to,
// 		Time:   time.Now().Unix(),
// 		Fee:    fee,
// 		Script: script,
// 	}
// 	tx.HashTransaction()

// 	return tx
// }

// HashTransaction 对交易进行hash
func (tx *Transaction) HashTransaction() {
	fromBytes := tx.From[:]
	toBytes := tx.To[:]
	nonceBytes := miscellaneous.E64func(tx.Nonce)
	amountBytes := miscellaneous.E64func(tx.Amount)
	timeBytes := miscellaneous.E64func(uint64(tx.Time))
	txBytes := bytes.Join([][]byte{nonceBytes, amountBytes, fromBytes, toBytes, timeBytes}, []byte{})
	hash := sha3.Sum256(txBytes)
	tx.Hash = hash[:]
}

// TrimmedCopy 有选择的对交易的字段进行拷贝，用以验证签名
func (tx *Transaction) TrimmedCopy() *Transaction {
	txCopy := &Transaction{
		Nonce:  tx.Nonce,
		Amount: tx.Amount,
		From:   tx.From,
		To:     tx.To,
		Time:   tx.Time,
	}
	return txCopy
}

// Sign 用ed25519椭圆曲线签名算法，对交易进行签名
func (tx *Transaction) Sign(privateKey []byte) {
	signatures := ed25519.Sign(ed25519.PrivateKey(privateKey), tx.Hash)
	tx.Signature = signatures
}

// Verify 验证签名，成功返回true，否则返回false
func (tx *Transaction) Verify() bool {
	txCopy := tx.TrimmedCopy()
	txCopy.HashTransaction()
	publicKey := tx.From.ToPublicKey()
	return ed25519.Verify(publicKey, txCopy.Hash, tx.Signature)
}

// EqualNonce 当前交易与传入的交易的nonce相同返回ture，否则返回ture
func (tx *Transaction) EqualNonce(transaction *Transaction) bool {
	if tx.From == transaction.From && tx.Nonce == transaction.Nonce {
		return true
	}
	return false
}
