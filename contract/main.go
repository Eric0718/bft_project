package main

import (
	"fmt"
	"kortho/contract/exec"
	"kortho/util/store/bg"
	"log"
)

func main() {
	db := bg.New("contract.db")
	defer db.Close()

	// 1.
	/* sc := parser.Parser([]byte("new \"USDT\" 1000 2"))
	e, err := exec.New(db, sc, "wxh")
	if err != nil {
		fmt.Println("===", err)
		return
	}
	fmt.Printf("%x\n", e.Root())
	e.Flush() */

	// {
	// 	sc1 := parser.Parser([]byte("mint \"USDT\" 10000"))
	// 	e, err := exec.New(db, sc1, "wxh")
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	fmt.Printf("%x\n", e.Root())
	// 	e.Flush()
	// }

	/* sc := parser.Parser([]byte("transfer \"USDT\" 20 \"lq\""))
	e, err := exec.New(db, sc, "lzl")
	if err != nil {
		fmt.Println("===", err)
		return
	}
	fmt.Printf("%x\n", e.Root())
	e.Flush() */

	/*
		sc := parser.Parser([]byte("transfer \"USDT\" 20 \"wxh\""))
		e, err := exec.New(db, sc, "zyh")
		if err != nil {
			fmt.Println("===", err)
			return
		}
		fmt.Printf("%x\n", e.Root())
		e.Flush()
	*/

	b, err := exec.Balance(db, "BEC", "Kto6RX3LA5rZcL936DaJBHZHzV1GLQDMzqShPWWB2ANr8q7") // 2，代比， 3，地址
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v\n", b)
	db.Close()

	/* b, err := exec.Precision(db, "USDT") // 2，代比， 3，地址
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v\n", b)
	db.Close() */
}
