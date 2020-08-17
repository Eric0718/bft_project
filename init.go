// package main

// import (
// 	"fmt"

// 	"kortho/util/miscellaneous"
// 	"kortho/util/store/bg"
// )

// func main() {

// 	db := bg.New("blockchain.db")
// 	tx := db.NewTransaction()

// 	h := miscellaneous.E64func(1)
// 	err := tx.Set([]byte("height"), h)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	//块hash->交易hash->交易数据
// 	err = tx.Mset([]byte("123456"), []byte("wangxihaoyuheying"), []byte("654321"))
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	//交易hash->交易数据->块高
// 	err = tx.Mset([]byte("wangxihaoyuheying"), []byte("654321"), h)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	//交易hash->交易数据
// 	err = tx.Set([]byte("wangxihaoyuheying"), []byte("654321"))
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	// "nonce" ->addr -> nonce
// 	addrs := []byte("CNtyeiE8RkTy26ufueMnvvbkEJ5qQL7tjD8Su5BP8PLY")
// 	tx.Mset([]byte("nonce"), addrs, miscellaneous.E64func(1))
// 	// addr -> balance
// 	err = tx.Set(addrs, miscellaneous.E64func(1000000000000))
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	tx.Commit()
// 	return

// }

// type Block struct {
// 	Height        uint64                     //当前块号
// 	PrevBlockHash []byte                     //上一块的hash
// 	Txs           []*transaction.Transaction //交易数据
// 	Root          []byte                     //默克根
// 	Version       uint64                     //版本号
// 	Timestamp     int64                      //时间戳
// 	Hash          []byte                     //当前块hash
// 	Miner         []byte
// 	Txres         map[types.Address]uint64

// }
//{1 [] [] [] 1 1577256099 [128 214 141 242 82 191 0 242 131 22 163 234 198 165 126 117 226 6 31 138 79 183 23 130 179 249 121 178 39 162 196 56] [52 66 56 82 84 1
//10 70 99 104 49 87 115 50 76 119 66 97 68 102 85 57 80 87 70 70 101 71 76 119 70 116 51 89 53 86 101 70 111 65 89 57 82 85 114] map[]}

package main

import (
	"crypto/sha256"
	"fmt"

	"kortho/block"
	"kortho/blockchain"
	"kortho/types"
	"kortho/util/merkle"
	"kortho/util/miscellaneous"
	"kortho/util/store"
	"kortho/util/store/bg"
)

func main01() {
	tree := merkle.New(sha256.New(), nil)
	root := tree.GetMtHash()
	fmt.Println("root =", root)
	//timetamp := time.Now().Unix()
	//txres := make(map[types.Address]uint64)
	miner := []byte("Kto9sFhbjDdjEHvcdH6n9dtQws1m4ptsAWAy7DhqGdrUFai")
	//Kto9sFhbjDdjEHvcdH6n9dtQws1m4ptsAWAy7DhqGdrUFai
	//5BWVgtMPPUPFuCHssYhXxFx2xVfQRTkB1EjKHKu1B1KxdVxD5cswDEdqiko3PjUPFPGfePoKxdfzHvv4YXRCYNp2

	addr, _ := types.BytesToAddress(miner)
	var newBlock block.Block = block.Block{
		Height:    0,
		Root:      root,
		Version:   1,
		Timestamp: 1577256099,
		Miner:     *addr,
	}
	newBlock.SetHash()
	fmt.Println(newBlock)
	AddBlock(&newBlock)
}

func AddBlock(b *block.Block) error {
	db := bg.New("blockchain.db")
	defer db.Close()

	tx := db.NewTransaction()
	defer tx.Cancel()

	//byte转64   块高
	// var h uint64 = 1
	// h++
	hash := b.Hash
	//高度->哈希
	err := tx.Set(append(blockchain.HeightPrefix, miscellaneous.E64func(b.Height)...), hash)
	//	err := tx.Mset(miscellaneous.E64func(1), []byte("hash"), hash)
	if err != nil {
		fmt.Println(err)
		return err
	}
	bt := b.Serialize()
	//哈希-> 块
	err = tx.Set(hash, bt)
	if err != nil {
		fmt.Println(err)
		return err
	}

	tx.Del([]byte("height"))
	tx.Set([]byte("height"), miscellaneous.E64func(b.Height))

	ar := b.Miner.Bytes()
	MinerAccount(tx, ar, 1000000000000000000)
	return tx.Commit()
}

func MinerAccount(tx store.Transaction, addr []byte, amount uint64) {
	tobalance, err := tx.Get(addr)
	if err != nil {
		set_balance(tx, addr, miscellaneous.E64func(0))
		tobalance = miscellaneous.E64func(0)
	}
	ToBalance, _ := miscellaneous.D64func(tobalance)
	ToBalance += amount
	Tobytes := miscellaneous.E64func(ToBalance)

	set_balance(tx, addr, Tobytes)
}

func set_balance(tx store.Transaction, addr, balance []byte) error {
	// tx := db.NewTransaction()
	// defer tx.Cancel()
	tx.Del(addr)
	err := tx.Set(addr, balance)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return err
}

// func (Bc *Blockchain) AddBlock(b *block.Block, mineraddr []byte) error {
// 	tx := Bc.db.NewTransaction()
// 	defer tx.Cancel()
// 	//拿出块高
// 	v, err := tx.Get([]byte("height"))
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}
// 	//byte转64   块高
// 	h, _ := miscellaneous.D64func(v)
// 	h++
// 	hash := b.Hash
// 	//高度->哈希

// 	err = tx.Mset(miscellaneous.E64func(h), []byte("hash"), hash)
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}
// 	bt := b.Serialize()
// 	//哈希-> 块
// 	err = tx.Set(hash, bt)
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}
// 	//高度对下个高度
// 	err = tx.Set(v, miscellaneous.E64func(h+1))
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}
// 	tx.Del([]byte("height"))
// 	tx.Set([]byte("height"), miscellaneous.E64func(h))
// 	txs := b.Tx()
// 	for _, Tx := range txs {
// 		if Tx.Script != "" {
// 			sc := parser.Parser([]byte(Tx.Script))
// 			e, err := exec.New(Bc.cdb, sc, string(Tx.From.Bytes()))
// 			if err != nil {
// 				Tx.Errmsg = "exec New is failed "
// 			}
// 			err = e.Flush()
// 			if err != nil {
// 				Tx.Errmsg = "flush code is failed"
// 			}
// 		}
// 		err := setTxbyaddr(tx, Tx.From.Bytes(), *Tx)
// 		if err != nil {
// 			fmt.Println(err)
// 			return err
// 		}
// 		err = setTxbyaddr(tx, Tx.To.Bytes(), *Tx)
// 		if err != nil {
// 			fmt.Println(err)
// 			return err
// 		}

// 		setAccount(tx, Tx.From.Bytes(), Tx.To.Bytes(), Tx.Amount, Tx.Nonce)
// 		tx_data, err := json.Marshal(Tx)
// 		if err != nil {
// 			fmt.Println("142======", err)
// 			return err
// 		}
// 		fmt.Println("145======", err)
// 		//块hash->交易hash->交易数据
// 		setBlockdata(tx, b.Hash, Tx.Hash, tx_data, miscellaneous.E64func(h))

// 	}

// 	ar := types.BytesToAddress(mineraddr)

// 	// amounts := new(big.Int)
// 	// amounts.SetString("56110000000", 10)
// 	MinerAccount(tx, ar.Bytes(), 56110000000)
// 	return tx.Commit()
// }
