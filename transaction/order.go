package transaction

import (
	"crypto/ed25519"
	"kortho/types"
)

// Order 订单数据结构
type Order struct {
	// ID 订单号，这是有外部的订单系统传入
	ID         []byte        `json:"id"`
	Address    types.Address `json:"address"`
	Price      uint64        `json:"price"`
	Hash       []byte        `json:"hash"`
	Ciphertext []byte        `json:"ciphertext"`
	Signature  []byte        `json:"signature"`
	Tradename  string        `json:"tradename"`
	Region     string        `json:"region"`
}

// SignOrder 用ed25519椭圆曲线签名算法对订单进行签名
func (o *Order) SignOrder(privateKey []byte) {
	o.Signature = ed25519.Sign(ed25519.PrivateKey(privateKey), o.Hash)
}

// Vertify 对签名进行验证
func (o *Order) Vertify(address types.Address) bool {
	publicKey := address.ToPublicKey()
	return ed25519.Verify(ed25519.PublicKey(publicKey), o.Hash, o.Signature)
}
