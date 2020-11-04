package blockchain

import (
	"fmt"
	"kortho/logger"
	"kortho/transaction"
	"kortho/types"
	"kortho/util/miscellaneous"
	"kortho/util/store"

	"go.uber.org/zap"
)

var (
	//锁仓投票标记
	Voteflag = []byte("TP||")
	//锁仓收益分红标记
	Sobflag = []byte("FH||")
	//锁仓总资金池
	sFundPool = []byte("samount")
	Miners    = []byte("miners")
	//社区分配金额
	//Samount   uint64
	damt      uint64
	IncomeAmt uint64
)

//锁仓竞选数据结构
type Vote struct {
	node    []byte
	amount  uint64
	address []byte
	miners  []byte
}

func NewVote(tx *transaction.Transaction) *Vote {
	v := &Vote{
		address: tx.From.Bytes(),
		node:    tx.Root,
		amount:  tx.Amount,
		miners:  tx.Signature,
	}
	return v
}

//社区收益
func SetIncome(amount uint64) {

	IncomeAmt += amount
	return

}

//锁仓收益分配

func (bc *Blockchain) ShareOutBouns(txs []*transaction.Transaction) error {

	bc.mu.RLock()
	defer bc.mu.RUnlock()
	tx := bc.db.NewTransaction()
	defer tx.Cancel()
	var i int
	var node []byte

	//投票总资金池
	amt, _ := tx.Get([]byte("sFundPool"))
	samt, _ := miscellaneous.D64func(amt)
	//	fmt.Println("sFundPool", samt)

	nodes, err := tx.Lrange(Sobflag, 0, tx.Llen(Sobflag))
	if err != nil {
		logger.Error("get node list error.")
		//		fmt.Println("获取分红节点列表出错")
		return err
	}
	//收益分配
	for i, node = range nodes {
		//		fmt.Println("分红节点[", i, "]=", string(node))
		logger.Error("分红节点", zap.Int("i", i), zap.String("node", string(node)))

		//获取该节点下的用户列表

		users, _ := tx.Lrange(append(Sobflag, node...), 0, tx.Llen(append(Sobflag, node...)))
		//1.普通用户

		for _, user := range users {
			//获取该用户分红周期内的锁仓金额

			amt, err := tx.Mget(Sobflag, user)
			if err != nil {
				logger.Error("Get  user-(frozen  amount)  err")
				//fmt.Println("获取用户分红金额出错'")
				return err
			}
			famt, _ := miscellaneous.D64func(amt)
			//
			//分红收益
			//
			//个人收益=(社区总收益（天）-damt)*75%*个人投票占比

			rat := float64(famt) / float64(samt)

			pamt := (float64(IncomeAmt) - float64(damt)) * 3 / 4 * rat
			//1.将收益转入用户地址
			buser, _ := types.BytesToAddress(user)
			txs = append(txs, transaction.NewCoinBaseTransaction(*buser, uint64(pamt)))

		}
		//
		//2.矿主收益分配
		//
		//获取节点-矿主    节点-资金池
		minerAddr, _ := tx.Get(append(Miners, node...))
		amt, err := tx.Get(append(Sobflag, node...))
		if err != nil {
			logger.Error("get node-pundpool error.")
			//	fmt.Println("获取分红节点出错")
			return err
		}
		amt1, _ := miscellaneous.D64func(amt)

		rat := float64(amt1) / float64(samt)

		pamt := (float64(IncomeAmt) - float64(damt)) / 4 * rat

		Maddr, _ := types.BytesToAddress(minerAddr)

		txs = append(txs, transaction.NewCoinBaseTransaction(*Maddr, uint64(pamt)))

	}
	//每天 的金额重置
	IncomeAmt = 0
	return nil
}

//投票周期结束，更新收益分配信息
func Voteresult(tx store.Transaction) {

	//删除分红信息
	nodes, err := tx.Lrange(Sobflag, 0, tx.Llen(Sobflag))
	//分红节点
	tx.Lclear(Sobflag)
	for _, node := range nodes {

		users, _ := tx.Lrange(append(Sobflag, node...), 0, tx.Llen(append(Sobflag, node...)))
		//删除分红  节点-用户
		tx.Lclear(append(Sobflag, node...))
		for _, user := range users {
			//删除 用户-金额
			tx.Mdel(Sobflag, user)
		}

	}

	//1.获取节点累计投票金额
	var t []*Vote
	var samount uint64

	logger.Info("---------------------Vote results are processed on a monthly basis--------------------------")
	//获取节点 -资金池
	nn, a, err := tx.Mkvs(Voteflag)
	if err != nil {
		logger.Error("Get  node-fundpool err")
		return
	}

	for _, n := range nn {
		v1 := &Vote{}
		v1.node = n
		t = append(t, v1)
	}

	for i, a1 := range a {
		am, _ := miscellaneous.D64func(a1)
		t[i].amount = am
	}

	//竞选节点根据金额排序
	for j := 0; j < len(t)-1; j++ {
		for k := j + 1; k < len(t); k++ {
			if t[j].amount < t[k].amount {
				t[j], t[k] = t[k], t[j]
			}
		}
	}
	/* 	处理竞选结果
	   	1.竞选成功节点，设置下个周期的分红信息,删除投票信息
	   	2.竞选失败节点，解锁锁仓金额，删除投票信息 */
	for i, v := range t {

		logger.Info("node   is", zap.String("node", string(v.node)))

		//删除投票信息
		defer tx.Mdel(Voteflag, v.node)              //投票   节点-资金池
		defer tx.Lclear(append(Voteflag, v.node...)) //投票   节点 -用户清单
		//
		//1.竞选成功节点，设置下个周期的分红信息,删除投票信息
		//前13个节点为竞选成功的节点
		//
		if i < 13 {

			//设置分红节点
			tx.Lrpush(Sobflag, v.node)
			//设置分红	节点--资金池
			//1.获取投票  节点--资金池
			/* amt, err := tx.Mget(Voteflag, v.node)
			if err != nil {
				logger.Error("Get  vote.node-fundpool (map)err")
				return
			}
			//2.设置分红信息   节点-资金池
			tx.Set(append(Sobflag, v.node...), amt)
			amt1, _ := miscellaneous.D64func(amt)
			samount += amt1 */

			samount += v.amount
			tx.Set(append(Sobflag, v.node...), miscellaneous.E64func(v.amount))

			//设置分红  节点--用户
			//1.获取投票   节点--用户
			users, _ := tx.Lrange(append(Voteflag, v.node...), 0, tx.Llen(append(Voteflag, v.node...)))
			tx.Lclear(append(Voteflag, v.node...))

			if users != nil {
				for _, u := range users {
					//删除个人投票信息   用户-金额
					defer tx.Del(append(Voteflag, u...))

					//分红信息    节点-用户
					tx.Lrpush(append(Sobflag, v.node...), u)

					//分红信息  用户-金额

					amt, err := tx.Get(append(Voteflag, u...))
					if err != nil {
						fmt.Println("获取用户投票金额err", err)
						logger.Error("Get user-amount (kv)err")
					} else {
						tx.Mset(Sobflag, u, amt)

					}
					am, _ := miscellaneous.D64func(amt)
					fmt.Println("用户[", string(u), "]累计投票，memory=", am)

				}
			} else {
				fmt.Println("The node no user")
			}

		} else {
			//竞选失败的节点回退投票数据
			//1.根据节点获取投票用户
			users, _ := tx.Lrange(append(Voteflag, v.node...), 0, tx.Llen(append(Voteflag, v.node...)))

			if users != nil {
				for _, user := range users {
					//竞选失败，更新冻结资金
					vamt, _ := tx.Get(append(Voteflag, user...))
					vamt1, _ := miscellaneous.D64func(vamt)
					tx.Del(append(Voteflag, user...))
					freeamt, _ := tx.Mget(FreezeKey, user)
					famt, _ := miscellaneous.D64func(freeamt)
					pamt := famt - vamt1
					tx.Mset(FreezeKey, user, miscellaneous.E64func(pamt))

				}
			}
		}
	}

	//设置分红资金池总额
	tx.Set([]byte("sFundPool"), miscellaneous.E64func(samount))

	logger.Info("sFundPool", zap.Uint64("sFundPool", samount))

}

/*保存投票信息*/
func SetVote(V Vote, tx store.Transaction) {

	logger.Info("===============Individual  voting statistics=============")

	logger.Info("vote mes:", zap.String("node", string(V.node)), zap.String("amt", string(V.amount)), zap.String("addr", string(V.address)))

	var vamt uint64

	if V.miners != nil {
		tx.Set(append(Miners, V.node...), V.address)
	}

	//1.获取该用户投票记录，有则投票金额累加，无则新增记录
	amt, Err := tx.Get(append(Voteflag, V.address...))
	if amt == nil {

		logger.Info("User is  first vote", zap.Error(Err))
		//投票信息  节点-用户
		tx.Llpush(append(Voteflag, V.node...), V.address)
		vamt = V.amount
	} else {
		amt1, _ := miscellaneous.D64func(amt)
		vamt = V.amount + amt1
	}
	//投票信息  用户-金额
	tx.Set(append(Voteflag, V.address...), miscellaneous.E64func(vamt))
	logger.Info("user [", zap.String("user:", string(V.address)), zap.Uint64("]voteing  total  amount :", vamt))

	logger.Info("==============node voting statistics================")

	//2.竞选节点，该节点第一次获得投票则新建节点信息，之后该节点的投票金额累计
	amt, err := tx.Mget(Voteflag, V.node)
	if err != nil {

		logger.Info("node first voting", zap.String("node", string(V.node)))
	} else {

		amt1, _ := miscellaneous.D64func(amt)
		V.amount += amt1
	}
	//投票信息   节点--资金池
	tx.Mset(Voteflag, V.node, miscellaneous.E64func(V.amount))

}
