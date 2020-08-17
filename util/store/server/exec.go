package server

import (
	"errors"
	"fmt"
	"kortho/util/store"
	"strconv"
)

func init() {
	dealRegister = make(map[string]dealFunc)

	dealRegister["SET"] = deal0
	dealRegister["DEL"] = deal1
	dealRegister["GET"] = deal2

	dealRegister["HSET"] = deal3
	dealRegister["HDEL"] = deal4
	dealRegister["HGET"] = deal5
	dealRegister["HKEYS"] = deal6
	dealRegister["HVALS"] = deal7
	dealRegister["HCLEAR"] = deal8

	dealRegister["SADD"] = deal9
	dealRegister["SREM"] = deal10
	dealRegister["SISMEMBER"] = deal11
	dealRegister["SMEMBERS"] = deal12
	dealRegister["SCLEAR"] = deal13

	dealRegister["ZADD"] = deal14
	dealRegister["ZREM"] = deal15
	dealRegister["ZSCORE"] = deal16
	dealRegister["ZRANGE"] = deal17
	dealRegister["ZCLEAR"] = deal18

	dealRegister["LPOP"] = deal19
	dealRegister["LPUSH"] = deal20
	dealRegister["RPOP"] = deal21
	dealRegister["RPUSH"] = deal22
	dealRegister["LSET"] = deal23
	dealRegister["LLEN"] = deal24
	dealRegister["LINDEX"] = deal25
	dealRegister["LRANGE"] = deal26
	dealRegister["LCLEAR"] = deal27
}

// SET k v
func deal0(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'SET' command"))
		return
	}
	if err := db.Set(args[0], args[1]); err != nil {
		resp.respError(err)
		return
	}
	resp.respSimple("OK")
}

// DEL k
func deal1(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'DEL' command"))
		return
	}
	if err := db.Del(args[0]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// GET k
func deal2(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'GET' command"))
		return
	}
	v, err := db.Get(args[0])
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(string(v))
	}
}

// HSET k field v
func deal3(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 3 {
		resp.respError(errors.New("wrong number of arguments for 'HSET' command"))
		return
	}
	if err := db.Mset(args[0], args[1], args[2]); err != nil {
		resp.respError(err)
		return
	}
	resp.respInteger(1)
}

// HDEL k field
func deal4(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'HDEL' command"))
		return
	}
	if err := db.Mdel(args[0], args[1]); err != nil {
		resp.respInteger(0)
		return
	}
	resp.respInteger(1)
}

// HGET k field
func deal5(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'HGET' command"))
		return
	}
	v, err := db.Mget(args[0], args[1])
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(string(v))
	}
}

// HKEYS k
func deal6(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'HKEYS' command"))
		return
	}
	ks, err := db.Mkeys(args[0])
	switch {
	case err == nil:
		resp.respArray(ks)
	default:
		resp.respArray([][]byte{})
	}
}

// HVALS k
func deal7(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'HVALS' command"))
		return
	}
	vs, err := db.Mvals(args[0])
	switch {
	case err == nil:
		resp.respArray(vs)
	default:
		resp.respArray([][]byte{})
	}
}

// HCLEAR k
func deal8(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'HCLEAR' command"))
		return
	}
	if err := db.Mclear(args[0]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// SADD k v
func deal9(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'SADD' command"))
		return
	}
	if err := db.Sadd(args[0], args[1]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// SREM k v
func deal10(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'SREM' command"))
		return
	}
	if err := db.Sdel(args[0], args[1]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// SISMEMBER k v
func deal11(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'SISMEMBER' command"))
		return
	}
	ok, _ := db.Selem(args[0], args[1])
	switch ok {
	case true:
		resp.respInteger(1)
	default:
		resp.respInteger(0)
	}
}

// SMEMBERS k
func deal12(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'SMEMBERS' command"))
		return
	}
	vs, err := db.Smembers(args[0])
	switch {
	case err == nil:
		resp.respArray(vs)
	default:
		resp.respArray([][]byte{})
	}
}

// SCLEAR k
func deal13(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'SMEMBERS' command"))
		return
	}
	if err := db.Sclear(args[0]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// ZADD k score v
func deal14(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 3 {
		resp.respError(errors.New("wrong number of arguments for 'ZADD' command"))
		return
	}
	score, err := strconv.ParseInt(string(args[1]), 0, 32)
	if err != nil {
		resp.respInteger(0)
		return
	}
	if err := db.Zadd(args[0], int32(score), args[2]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// ZREM k v
func deal15(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'ZREM' command"))
		return
	}
	if err := db.Zdel(args[0], args[1]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// ZSCORE k v
func deal16(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'ZSCORE' command"))
		return
	}
	score, err := db.Zscore(args[0], args[1])
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(fmt.Sprintf("%v", score))
	}
}

// ZRANGE k start stop
func deal17(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 3 {
		resp.respError(errors.New("wrong number of arguments for 'ZRANGE' command"))
		return
	}
	start, err := strconv.ParseInt(string(args[1]), 0, 32)
	if err != nil {
		resp.respError(err)
		return
	}
	stop, err := strconv.ParseInt(string(args[2]), 0, 32)
	if err != nil {
		resp.respError(err)
		return
	}
	vs, err := db.Zrange(args[0], int32(start), int32(stop))
	switch {
	case err == nil:
		resp.respArray(vs)
	default:
		resp.respError(err)
	}
}

// ZCLEAR k
func deal18(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'ZCLEAR' command"))
		return
	}
	if err := db.Zclear(args[0]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}

// LPOP k
func deal19(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'LPOP' command"))
		return
	}
	v, err := db.Llpop(args[0])
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(string(v))
	}
}

// LPUSH k v
func deal20(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'LPUSH' command"))
		return
	}
	if n, err := db.Llpush(args[0], args[1]); err != nil {
		resp.respError(err)
	} else {
		resp.respInteger(int(n))
	}
}

// RPOP k
func deal21(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'RPOP' command"))
		return
	}
	v, err := db.Lrpop(args[0])
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(string(v))
	}
}

// RPUSH k v
func deal22(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'RPUSH' command"))
		return
	}
	if n, err := db.Lrpush(args[0], args[1]); err != nil {
		resp.respError(err)
	} else {
		resp.respInteger(int(n))
	}
}

// LSET k index v
func deal23(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 3 {
		resp.respError(errors.New("wrong number of arguments for 'LSET' command"))
		return
	}
	idx, err := strconv.ParseInt(string(args[1]), 0, 64)
	if err != nil {
		resp.respError(err)
		return
	}
	if err := db.Lset(args[0], idx, args[2]); err != nil {
		resp.respError(err)
		return
	}
	resp.respSimple("OK")
}

// LLEN k
func deal24(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'LLEN' command"))
		return
	}
	resp.respInteger(int(db.Llen(args[0])))
}

// LINDEX k index
func deal25(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 2 {
		resp.respError(errors.New("wrong number of arguments for 'LINDEX' command"))
		return
	}
	idx, err := strconv.ParseInt(string(args[1]), 0, 64)
	if err != nil {
		resp.respError(err)
		return
	}
	v, err := db.Lindex(args[0], idx)
	switch {
	case err != nil:
		resp.respBulk("")
	default:
		resp.respBulk(string(v))
	}
}

// LRANGE k start stop
func deal26(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 3 {
		resp.respError(errors.New("wrong number of arguments for 'LRANE' command"))
		return
	}
	start, err := strconv.ParseInt(string(args[1]), 0, 64)
	if err != nil {
		resp.respError(err)
		return
	}
	stop, err := strconv.ParseInt(string(args[2]), 0, 64)
	if err != nil {
		resp.respError(err)
		return
	}
	vs, err := db.Lrange(args[0], start, stop)
	switch {
	case err == nil:
		resp.respArray(vs)
	default:
		resp.respArray([][]byte{})
	}
}

// LCLEAR k
func deal27(db store.DB, resp responseWriter, args [][]byte) {
	if len(args) < 1 {
		resp.respError(errors.New("wrong number of arguments for 'LCLEAR' command"))
		return
	}
	if err := db.Lclear(args[0]); err != nil {
		resp.respInteger(0)
	} else {
		resp.respInteger(1)
	}
}
