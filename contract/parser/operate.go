package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
)

func init() {
	operateRegistry = make(map[string]scriptOperateFunc)

	operateRegistry["new"] = operate0
	operateRegistry["mint"] = operate1
	operateRegistry["transfer"] = operate2

	operateRegistry["freeze"] = operate3
	operateRegistry["unfreeze"] = operate4

	/*
		operateRegistry["rate"] = operate5
		operateRegistry["post"] = operate6
	*/
}

func Parser(data []byte) []*Script {
	var xs []*Script

	wp := NewParser(bytes.NewReader(data))
	if err := wp.Parser(); err != nil {
		return nil
	}
	for i, j := 0, len(wp.ns); i < j; i++ {
		name, _ := wp.ns[0].left.value.(string)
		if fp, ok := operateRegistry[name]; !ok {
			return nil
		} else {
			xs = append(xs, fp(wp.ns[0].right))
		}
	}
	return xs
}

// new tokenId total_amount
func operate0(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(AMOUNT, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "new",
	}
}

// mint tokenId amount
func operate1(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(AMOUNT, np, &args); err != nil { // amount
		return nil
	}
	return &Script{
		args: args,
		name: "mint",
	}
}

// transfer tokenId amount address
func operate2(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(AMOUNT, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(ADDRESS, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "transfer",
	}
}

// freeze tokenId address
func operate3(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(ADDRESS, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "freeze",
	}
}

// unfreeze tokenId address
func operate4(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(ADDRESS, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "unfreeze",
	}
}

// rate tokenId percentage
func operate5(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(PERCENTAGE, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "rate",
	}
}

// post tokenId order
func operate6(np *scriptNode) *Script {
	var args []*ScriptArgument

	if np == nil {
		return nil
	}
	if err := takeArgument(TOKENID, np, &args); err != nil {
		return nil
	}
	if np = nextArgument(np); np == nil {
		return nil
	}
	if err := takeArgument(ORDER, np, &args); err != nil {
		return nil
	}
	return &Script{
		args: args,
		name: "post",
	}
}

func takeArgument(typ uint32, np *scriptNode, args *[]*ScriptArgument) error {
	arg := argumentValue(np)
	if (arg.op&SUB_MASK)>>SUB_OFF != typList[typ] {
		return errors.New("type mismatch")
	}
	*args = append(*args, &ScriptArgument{typ, arg.value})
	return nil
}

func argumentValue(np *scriptNode) *scriptNode {
	switch np.op & TYP_MASK {
	case O_ELIST:
		return np.left
	default:
		return np
	}
}

func nextArgument(np *scriptNode) *scriptNode {
	switch np.op & TYP_MASK {
	case O_ELIST:
		return np.right
	default:
		return nil
	}
}

func dup(a []byte) []byte {
	if a == nil {
		return nil
	}
	b := []byte{}
	return append(b, a...)
}

func e32func(a uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, a)
	return buf
}

func d32func(a []byte) (uint32, error) {
	if len(a) != 4 {
		return 0, errors.New("D32func: Illegal slice length")
	}
	return binary.LittleEndian.Uint32(a), nil
}

func e64func(a uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, a)
	return buf
}

func d64func(a []byte) (uint64, error) {
	if len(a) != 8 {
		return 0, errors.New("D64func: Illegal slice length")
	}
	return binary.LittleEndian.Uint64(a), nil
}

// slice length || slice
func eslice(a []byte) []byte {
	buf := []byte{}
	buf = append(buf, e32func(uint32(len(a)))...)
	buf = append(buf, a...)
	return buf
}

func dslice(data []byte) ([]byte, []byte, error) {
	if len(data) < 4 {
		return []byte{}, []byte{}, errors.New("DecodeSlice: Illegal slice length")
	}
	n, _ := d32func(data[:4])
	if data = data[4:]; uint32(len(data)) < n {
		return []byte{}, []byte{}, errors.New("DecodeSlice: Illegal slice length")
	}
	return dup(data[:n]), data[n:], nil
}
