package server

import (
	"kortho/util/store"
	"net"
	"net/textproto"
)

type Server interface {
	Run()
	Stop()
}

type responseWriter interface {
	respError(error)
	respInteger(int)
	respBulk(string)
	respSimple(string)
	respArray([][]byte)
}

type server struct {
	db  store.DB
	lis net.Listener
	ch  chan struct{}
}

type request struct {
	args [][]byte
}

type response struct {
	w *textproto.Writer
}

type dealFunc (func(store.DB, responseWriter, [][]byte))

var dealRegister map[string]dealFunc
