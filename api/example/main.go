package main

import (
	"context"
	"fmt"
	"kortho/api/message"
	"kortho/transaction"
	"kortho/types"
	"log"
	"time"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("127.0.0.1:8501", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	client := message.NewGreeterClient(conn)

	fWallet, tWallet := types.NewWallet(), types.NewWallet()

	from, _ := types.StringToAddress(fWallet.Address)
	to, _ := types.StringToAddress(tWallet.Address)

	nonceResp, err := client.GetAddressNonceAt(context.Background(), &message.ReqNonce{Address: from.String()})
	if err != nil {
		log.Fatal(err)
	}

	tx := &transaction.Transaction{
		From:   *from,
		To:     *to,
		Amount: 100000,
		Nonce:  nonceResp.Nonce,
		Time:   time.Now().Unix(),
	}
	tx.HashTransaction()
	tx.Sign(fWallet.PrivateKey)

	req := &message.ReqSignedTransaction{
		From:      tx.From.String(),
		To:        tx.To.String(),
		Amount:    tx.Amount,
		Nonce:     tx.Nonce,
		Time:      tx.Time,
		Hash:      tx.Hash,
		Signature: tx.Signature,
	}
	resp, err := client.SendSignedTransaction(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("hash:", resp.GetHash())
}
