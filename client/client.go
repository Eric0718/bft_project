package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"kortho/api/message"
	"kortho/transaction"
	"kortho/types"
	"kortho/util"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
)

type CLI struct {
	message.GreeterClient
}

func (c *CLI) getConn() {
	// certPool := x509.NewCertPool()
	// ca, err := ioutil.ReadFile("../configs/ca.crt")
	// if err != nil {
	// 	fmt.Printf("ioutil.ReadFile:%s\n", err.Error())
	// 	os.Exit(-1)
	// }
	// if !certPool.AppendCertsFromPEM(ca) {
	// 	fmt.Printf("certPool.AppendCertsFromPEM not ok\n")
	// 	os.Exit(-1)
	// }

	// creds := credentials.NewTLS(&tls.Config{
	// 	ServerName: "kortho.io",
	// 	RootCAs:    certPool,
	// })

	//conn, err := grpc.Dial("106.12.186.114:9501", grpc.WithTransportCredentials(creds))
	conn, err := grpc.Dial("106.12.186.114:8501", grpc.WithInsecure(), grpc.WithTimeout(30*time.Second))
	//conn, err := grpc.Dial("106.12.186.114:9501", grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Panic(err)
	}
	c.GreeterClient = message.NewGreeterClient(conn)
}

func isVaildArgs() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}
}

func printUsage() {
	fmt.Println("Usage:")

	fmt.Println("    wallet:\n\t-n\t创建n个钱包")
	fmt.Println("    send:\n\t-xfer\t交易数据json格式\n\t-frz\t冻结交易的json格式\n\t-unfrz\t解冻交易的json格式")
	fmt.Println("    get:\n\t-frz\t获取已冻结的金额")
}

func (c *CLI) Run() {
	isVaildArgs()

	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	walletCmd := flag.NewFlagSet("wallet", flag.ExitOnError)
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)

	number := walletCmd.Uint("n", 1, "钱包的个数")
	xferData := sendCmd.String("xfer", "", "交易的json格式")
	frzData := sendCmd.String("frz", "", "冻结请求的json格式")
	unfrzData := sendCmd.String("unfrz", "", "解冻请求的json格式")
	freezeAddr := getCmd.String("frz", "", "获取冻结金额的地址")

	switch os.Args[1] {
	case "send":
		c.getConn()
		if err := sendCmd.Parse(os.Args[2:]); err != nil {
			log.Panic(err)
		}
	case "wallet":
		if err := walletCmd.Parse(os.Args[2:]); err != nil {
			log.Panic(err)
		}
	case "get":
		c.getConn()
		if err := getCmd.Parse(os.Args[2:]); err != nil {
			log.Panic(err)
		}
	default:
		fmt.Printf("输入参数有误\n")
	}

	if sendCmd.Parsed() {
		if len(*xferData) != 0 {
			var req message.ReqTransaction
			if err := json.Unmarshal([]byte(*xferData), &req); err != nil {
				log.Panic(err)
			}
			c.sendTransaction(&req)
		} else if len(*frzData) != 0 {
			c.freezeBalance(*frzData)
		} else if len(*unfrzData) != 0 {
			c.unFreezeTransaction(*unfrzData)
		}
	}

	if walletCmd.Parsed() {
		c.getWallet(int(*number))
	}

	if getCmd.Parsed() {
		if len(*freezeAddr) != 0 {
			c.getFreezeBalance(*freezeAddr)
		}
	}
}

func (c *CLI) sendTransaction(req *message.ReqTransaction) {
	nonceResp, err := c.GetAddressNonceAt(context.Background(), &message.ReqNonce{Address: req.From})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("get nonce:", nonceResp.Nonce)

	req.Nonce = nonceResp.Nonce
	txResp, err := c.SendTransaction(context.Background(), req)
	if err != nil {
		fmt.Printf("send xfer error:%s\n", err)
		return
	}

	fmt.Printf("transaction hash:%s\n", txResp.Hash)
}

func (c *CLI) getWallet(number int) {
	for i := 1; i <= number; i++ {
		wallet := types.NewWallet()
		fmt.Printf("%d:\n", i)
		fmt.Printf("\t%s\n\t%s\n", wallet.Address,
			util.Encode(wallet.PrivateKey))
	}
}

func (c *CLI) freezeBalance(freezeTransaction string) {

	reqNonce := message.ReqNonce{
		Address: "Kto2YGvFKXQtSazWp9hPZyBrA9JPkxgNE6GW56o7jcdQXTq",
	}
	nonceResp, err := c.GetAddressNonceAt(context.TODO(), &reqNonce)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("get nonce:", nonceResp.Nonce)
	var freezeTx message.ReqSignedTransaction
	if err := json.Unmarshal([]byte(freezeTransaction), &freezeTx); err != nil {
		log.Panic(err)
	}
	from, _ := types.StringToAddress("Kto2YGvFKXQtSazWp9hPZyBrA9JPkxgNE6GW56o7jcdQXTq")
	to, err := types.StringToAddress(freezeTx.To)
	if err != nil {
		log.Fatal(err)
	}

	tx := &transaction.Transaction{
		From:   *from,
		To:     *to,
		Amount: freezeTx.Amount,
		Nonce:  nonceResp.Nonce,
		Time:   time.Now().Unix(),
	}
	tx.HashTransaction()

	privateKey := util.Decode("YmrjFawyRbczN91WqQkQpEqr5GeVek4hFMrLEsQ9EuUGi2znJ12xS2EbUA1E5gz4yEMyZVMa1uEyz76UxGA1ZuD")
	tx.Sign(privateKey)
	if !tx.Verify() {
		log.Fatalf("failed to verify\n")
	}

	freezeTx.Time = tx.Time
	freezeTx.Nonce = tx.Nonce
	freezeTx.Hash = tx.Hash
	freezeTx.Signature = tx.Signature

	var req message.ReqSignedTransactions
	req.Txs = append(req.Txs, &freezeTx)

	txResp, err := c.SendFreezeTransactions(context.Background(), &req)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("transaction hash:%s\n", txResp.HashList)
}

func (c *CLI) getFreezeBalance(address string) {
	resp, err := c.GetFreezeBalance(context.Background(), &message.ReqGetFreezeBal{AddressList: []string{address}})
	if err != nil {
		log.Fatal(err)
	}

	for _, result := range resp.Results {
		fmt.Printf("adddress:%s,balance:%d,state:%d\n", result.GetAddress(), result.GetBalance(), result.GetState())
	}
}

func (c *CLI) unFreezeTransaction(unfreezeTransaction string) {

	reqNonce := message.ReqNonce{
		Address: "Kto2YGvFKXQtSazWp9hPZyBrA9JPkxgNE6GW56o7jcdQXTq",
	}
	nonceResp, err := c.GetAddressNonceAt(context.TODO(), &reqNonce)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("get nonce:", nonceResp.Nonce)
	var freezeTx message.ReqSignedTransaction

	if err := json.Unmarshal([]byte(unfreezeTransaction), &freezeTx); err != nil {
		log.Panic(err)
	}

	from, _ := types.StringToAddress("Kto2YGvFKXQtSazWp9hPZyBrA9JPkxgNE6GW56o7jcdQXTq")
	to, err := types.StringToAddress(freezeTx.To)
	if err != nil {
		log.Fatal(err)
	}

	tx := &transaction.Transaction{
		From:   *from,
		To:     *to,
		Amount: freezeTx.Amount,
		Nonce:  nonceResp.Nonce,
		Time:   time.Now().Unix(),
	}
	tx.HashTransaction()

	privateKey := util.Decode("YmrjFawyRbczN91WqQkQpEqr5GeVek4hFMrLEsQ9EuUGi2znJ12xS2EbUA1E5gz4yEMyZVMa1uEyz76UxGA1ZuD")
	tx.Sign(privateKey)

	freezeTx.Time = tx.Time
	freezeTx.Nonce = tx.Nonce
	freezeTx.Hash = tx.Hash
	freezeTx.Signature = tx.Signature

	var req message.ReqSignedTransactions
	req.Txs = append(req.Txs, &freezeTx)

	txResp, err := c.SendUnfreezeTransactions(context.Background(), &req)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("transaction hash:%s\n", txResp.HashList)
}
