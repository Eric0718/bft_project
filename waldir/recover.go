package main

import (
	"encoding/json"
	"fmt"
	"kortho/block"
	"kortho/blockchain"
	"log"
	"os"
	"strconv"

	"github.com/coreos/etcd/wal"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal/walpb"
)

func Recovery(args []string) {
	var h uint64 = 1
	if len(args) < 1 {
		fmt.Println("Wrong number of parameters: should >=1 and <= 2!")
		return
	}
	if len(args) > 2 {
		fmt.Println("Too many parameters: should >=1 and <= 2!")
		return
	}
	var hi int = 1
	if len(args) == 2 {
		hi, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("strconv.Atoi err :", err)
			return
		}
		if hi < 1 {
			fmt.Println("Wrong given height:", hi)
			return
		}
	}

	bc := blockchain.New()
	mp := make(map[uint64]bool)
	walsnap := walpb.Snapshot{}
	w, err := wal.Open("./log/wal", walsnap)
	if err != nil {
		log.Fatalf("error loading wal (%v)", err)
	}
	_, _, ents, err := w.ReadAll()
	if err != nil {
		log.Fatalf("failed to read WAL (%v)", err)
	}
	var lastHeight uint64 = 1
	var lastTerm uint64 = 1
	var lastIndex uint64 = 1
	for i, e := range ents {
		if e.Type == raftpb.EntryNormal {
			if len(e.Data) > 0 {
				b := &block.Block{}
				if err := json.Unmarshal(e.Data, b); err != nil {
					fmt.Printf("failed to Unmarshal e.Data: %v\n", err)
					continue
				}
				switch args[0] {
				case "-r", "-R":
					if b.Height == uint64(hi) {
						fmt.Printf("Already recovered data to height: %v.\n", b.Height)
						return
					}

					if _, ok := mp[b.Height]; ok {
						fmt.Printf("%v\n", b.Height)
					} else {
						mp[b.Height] = true
						if h+1 != b.Height {
							log.Fatalf("%v: %v, %v\n", i, h, b.Height)
						}
						if err := bc.AddBlock(b, nil); err != nil {
							log.Fatalf("failed to add %v: %v\n", h, err)
						}
						h++
					}
				case "-h", "-H":
					if i > 0 {
						if b.Height != lastHeight+1 {
							fmt.Printf("last Index = %v,last Term = %v,last Height = %v.\n", lastIndex, lastTerm, lastHeight)
							fmt.Printf("current Index = %v,current Term = %v,current Height = %v.\n", e.Index, e.Term, b.Height)
							panic("Block height is discontinued!")
						}
					}

					lastHeight = b.Height
					lastIndex = e.Index
					lastTerm = e.Term
					// sData := fmt.Sprintf("Index = %s,Term = %s,Height = %s.", e.Index, e.Term, b.Height)
					// writeInfo(sData)
				}
			}
		}

	}
}
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Wrong parameter！")
		return
	}
	Recovery(os.Args[1:])
}

// func Recovery() {
// 	f, err := os.Create("test.txt")
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	walsnap := walpb.Snapshot{}
// 	w, err := wal.Open("./wal", walsnap)
// 	if err != nil {
// 		log.Fatalf("error loading wal (%v)", err)
// 	}
// 	_, _, ents, err := w.ReadAll()
// 	if err != nil {
// 		log.Fatalf("failed to read WAL (%v)", err)
// 	}
// 	l := len(ents) - 1
// 	for ; l > 0; l-- {
// 		if ents[l].Type == raftpb.EntryNormal {
// 			if len(ents[l].Data) > 0 {
// 				var bl block.Blocks
// 				err := json.Unmarshal(ents[l].Data, &bl)
// 				if err != nil {
// 					continue
// 				}
// 				if bl.Height == 1208 {

// 					for _, t := range bl.Txs {

// 						s := hex.EncodeToString(t.Hash)
// 						l, err := f.WriteString(s + "\n")
// 						if err != nil {
// 							fmt.Println(err)
// 							f.Close()
// 							return
// 						}
// 						fmt.Println(l, "bytes written successfully")
// 					}
// 					index := strconv.Itoa(int(ents[l].Index))
// 					f.WriteString(index + "\n")
// 					break
// 				}

// 			}

// 		}
// 	}
// 	err = f.Close()
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// }

func writeInfo(st string) {
	fp, err := os.OpenFile("./conf/walHeightTerm.json", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	_, err = fp.WriteString(st + "\n\r")
	if err != nil {
		log.Fatal(err)
	}
}
