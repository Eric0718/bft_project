package txpool

import (
	"kortho/logger"
	"kortho/transaction"
	"kortho/types"
)

const (
	// MaxAddrCount txpool最多能接受的交易数量
	MaxAddrCount = 1000
)

// TxHeap 是交易的指针切片，实现了container/heap中的heap接口
type TxHeap []*transaction.Transaction

// Len 获取txheap的长度
func (h TxHeap) Len() int { return len(h) }

// Less 索引i的元素小于索引j元素，返回ture,否则返回false
func (h TxHeap) Less(i, j int) bool {
	if h[i].From == h[j].From {
		if h[i].GetNonce() == h[j].GetNonce() {
			return h[i].GetTime() < h[j].GetTime()
		}
		return h[i].GetNonce() < h[j].GetNonce()
	}

	return h[i].GetTime() < h[j].GetTime()
}

func (h TxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push 向txHeap中添加元素
func (h *TxHeap) Push(x interface{}) {
	*h = append(*h, x.(*transaction.Transaction))
}

// Pop 弹出元素，弹出后从TxHeap中移除
func (h *TxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Get 获取下标为i的元素
func (h *TxHeap) Get(i int) interface{} {
	old := *h
	n := len(old)
	x := old[n-i]
	return x
}

func (h *TxHeap) check(fromAddr types.Address, nonce uint64) bool {
	var count = 0
	for _, tx := range *h {
		if fromAddr == tx.From {
			if nonce == tx.Nonce {
				logger.Error("txpool check:nonce == tx.Nonce")
				return false
			}
			count++
		}
	}

	if count >= MaxAddrCount {
		logger.Error("txpool check:count >= MaxAddrCount")
		return false
	}
	return true
}
