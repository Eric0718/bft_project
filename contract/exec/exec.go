package exec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"kortho/contract/parser"
	"sort"

	"kortho/util/merkle"
	"kortho/util/store"

	"golang.org/x/crypto/sha3"
)

const (
	DEBUG = false
)

var dealRegistry map[string]scriptDealFunc

func init() {
	dealRegistry = make(map[string]scriptDealFunc)

	dealRegistry["new"] = deal0
	dealRegistry["mint"] = deal1
	dealRegistry["transfer"] = deal2

	dealRegistry["freeze"] = deal3
	dealRegistry["unfreeze"] = deal4

	/*
		dealRegistry["rate"] = deal5
		dealRegistry["post"] = deal6
	*/
}

func Balance(db store.DB, id string, addr string) (uint64, error) {
	v, err := db.Get(bKey(id, addr))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(v), nil
}

func Precision(db store.DB, id string) (uint64, error) {
	v, err := db.Get(pKey(id))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(v), nil
}

func New(db store.DB, scs []*parser.Script, owner string) (*exec, error) {
	mp := make(map[string]string)
	for i, j := 0, len(scs); i < j; i++ {
		if err := dealRegistry[scs[i].Name()](db, owner, mp, scs[i]); err != nil {
			return nil, err
		}
	}
	return &exec{db, mp}, nil
}

func (e *exec) Root() []byte {
	var ss []string
	var xs [][]byte

	for k, _ := range e.mp {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	for _, k := range ss {
		xs = append(xs, []byte(k))
		xs = append(xs, []byte(e.mp[k]))
	}
	return merkle.New(sha3.New256(), xs).GetMtHash()
}

func (e *exec) Flush() error {
	tx := e.db.NewTransaction()
	defer tx.Cancel()
	for k, v := range e.mp {
		if err := tx.Set([]byte(k), []byte(v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// new tokenId total_amount precision
func deal0(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64) // total_amount
	arg2, _ := sc.Arguments()[2].Value().(uint64) // precision
	{
		k := aKey(arg0)
		if _, ok := mp[string(k)]; ok {
			return errors.New("token exist")
		}
		if _, err := db.Get(k); err == nil {
			return errors.New("token exist")
		}
	}
	mp[arg0] = executor
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, 0)
		mp[string(bKey(arg0, executor))] = string(buf)
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, arg1)
		mp[string(tKey(arg0, executor))] = string(buf)
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, arg2)
		mp[string(pKey(arg0))] = string(buf)
	}
	mp[string(fKey(arg0, executor))] = string([]byte{0})
	return nil
}

// mint tokenId amount
func deal1(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64) // amount
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	{
		var c, t uint64

		{
			k := tKey(arg0, executor)
			if v, ok := mp[string(k)]; ok {
				t = binary.LittleEndian.Uint64([]byte(v))
			} else {
				w, err := db.Get(k)
				switch {
				case err == nil:
					t = binary.LittleEndian.Uint64(w)
				case err == store.NotExist:
					t = 0
				default:
					return err
				}
			}
		}
		k := cKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			c = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := db.Get(k)
			switch {
			case err == nil:
				c = binary.LittleEndian.Uint64(w)
			case err == store.NotExist:
				c = 0
			default:
				return err
			}
		}
		if int64(c) > int64(t)-int64(arg1) {
			return errors.New("overflow")
		}
		{
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, c+arg1)
			mp[string(cKey(arg0, executor))] = string(buf)
		}
	}
	var b uint64
	{
		k := bKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			b = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := db.Get(k)
			switch {
			case err == nil:
				b = binary.LittleEndian.Uint64(w)
			case err == store.NotExist:
				b = 0
			default:
				return err
			}
		}
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, b+arg1)
		mp[string(bKey(arg0, executor))] = string(buf)
	}
	return nil
}

// transfer tokenId amount address
func deal2(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	var from, to uint64

	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64) // amount
	arg2, _ := sc.Arguments()[2].Value().(string) // address
	{
		k := aKey(arg0)
		if _, ok := mp[string(k)]; !ok {
			if _, err := db.Get(k); err != nil {
				return err
			}
		}
	}
	/*
		{
			k := fKey(arg0, executor)
			if v, ok := mp[string(k)]; ok {
				if bytes.Compare([]byte(v), []byte{1}) == 0 {
					return errors.New("freezed")
				}
			} else {
				w, err := db.Get(k)
				if err != nil {
					return err
				}
				if bytes.Compare(w, []byte{1}) == 0 {
					return errors.New("freezed")
				}
			}
		}
	*/
	{
		k := bKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			from = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := db.Get(k)
			switch {
			case err == nil:
				from = binary.LittleEndian.Uint64(w)
			case err == store.NotExist:
				from = 0
			default:
				return err
			}
		}
	}
	if from < arg1 {
		return errors.New("insufficient balance")
	}
	{
		k := bKey(arg0, arg2)
		if v, ok := mp[string(k)]; ok {
			to = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := db.Get(k)
			switch {
			case err == nil:
				to = binary.LittleEndian.Uint64(w)
			case err == store.NotExist:
				to = 0
			default:
				return err
			}
		}
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, from-arg1)
		mp[string(bKey(arg0, executor))] = string(buf)
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, to+arg1)
		mp[string(bKey(arg0, arg2))] = string(buf)
	}
	return nil
}

// freeze tokenId address
func deal3(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(string) // address
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	mp[string(fKey(arg0, arg1))] = string([]byte{1})
	return nil
}

// unfreeze tokenId address
func deal4(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(string) // address
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	mp[string(fKey(arg0, arg1))] = string([]byte{0})
	return nil
}

// rate
func deal5(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	return nil
}

// post
func deal6(db store.DB, executor string, mp map[string]string, sc *parser.Script) error {
	return nil
}

func aKey(id string) []byte {
	return []byte(id)
}

func bKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("b")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func pKey(id string) []byte {
	var buf bytes.Buffer

	buf.WriteString("p")
	buf.WriteString("/")
	buf.WriteString(id)
	return buf.Bytes()
}

func fKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("f")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func tKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("t")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func cKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("c")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}
