package main

import (
	"kortho/util/store/bg"
	"kortho/util/store/server"
)

func main() {
	db := bg.New("test.db")
	s := server.New(6378, db)
	s.Run()
}
