package exec

import (
	"kortho/contract/parser"

	"kortho/util/store"
)

type Exec interface {
	Root() []byte
	Flush() error
}

type exec struct {
	db store.DB
	mp map[string]string
}

type scriptDealFunc (func(store.DB, string, map[string]string, *parser.Script) error)
