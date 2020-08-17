package main

import (
	"kortho/util/store/bg"
)

func main() {
	db := bg.New("test.db")
	defer db.Close()

	// sc := parser.Parser([]byte("new \"BTC\" 1000000000"))
	// e, err := exec.New(db, sc, "wxh")
	// if err != nil {
	// 	fmt.Println("===", err)
	// 	return
	// }
	// fmt.Printf("%x\n", e.Root())
	// e.Flush()

	// sc1 := parser.Parser([]byte("mint \"BTC\" 1000000000"))
	// e, err := exec.New(db, sc1, "wxh")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Printf("%x\n", e.Root())
	// e.Flush()

	// // "transfer \"abc\" 10 \"to\""
	// sc1 := parser.Parser([]byte("transfer \"BTC\" 10 \"to\""))
	// e1, err := exec.New(db, sc1, "wxh")
	// if err != nil {
	// 	fmt.Println("===", err)
	// 	return
	// }
	// fmt.Printf("%x\n", e1.Root())
	// e1.Flush()

	// b, err := exec.Balance(db, "BTC", "wxh") // 2，代比， 3，地址
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Printf("%v\n", b)
}
