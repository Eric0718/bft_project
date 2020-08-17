package parser

import (
	"errors"
	"fmt"
)

const (
	ORDER = iota
	AMOUNT
	ADDRESS
	TOKENID
	PERCENTAGE
)

const (
	SCRIPT_TYPE_SIZE   = 4
	SCRIPT_COUNT_SIZE  = 4
	SCRIPT_UINT64_SIZE = 8
)

var typList = []int{
	O_STRING,
	O_INT,
	O_STRING, O_STRING,
	O_INT,
}

type scriptOperateFunc (func(*scriptNode) *Script)

var operateRegistry map[string]scriptOperateFunc

type ScriptArgument struct {
	typ   uint32
	value interface{}
}

type Script struct {
	name string
	args []*ScriptArgument
}

func (a *ScriptArgument) Show() []byte {
	buf := []byte{}
	buf = append(buf, e32func(a.typ)...)
	switch v := a.value.(type) {
	case uint64:
		buf = append(buf, e64func(v)...)
	case string:
		buf = append(buf, eslice([]byte(v))...)
	}
	return buf
}

func (a *ScriptArgument) Read(data []byte) ([]byte, error) {
	var err error

	if len(data) < SCRIPT_TYPE_SIZE {
		return []byte{}, errors.New("ScriptArgument Read: Illegal Length")
	}
	a.typ, _ = d32func(data[:SCRIPT_TYPE_SIZE])
	switch a.typ {
	case AMOUNT, PERCENTAGE:
		if data = data[SCRIPT_TYPE_SIZE:]; len(data) < SCRIPT_UINT64_SIZE {
			return []byte{}, errors.New("ScriptArgument Read: Illegal Length")
		}
		a.value, _ = d64func(data[:SCRIPT_UINT64_SIZE])
		data = data[SCRIPT_UINT64_SIZE:]
	case ORDER, ADDRESS, TOKENID:
		value := []byte{}
		if value, data, err = dslice(data[SCRIPT_TYPE_SIZE:]); err != nil {
			return []byte{}, fmt.Errorf("ScriptArgument Read: %v", err)
		} else {
			a.value = string(value)
		}
	}
	return data, nil
}

// args count || typ || name || args
func (a *Script) Show() []byte {
	buf := []byte{}
	n := uint32(len(a.args))
	buf = append(buf, e32func(n)...)
	buf = append(buf, eslice([]byte(a.name))...)
	for i := uint32(0); i < n; i++ {
		buf = append(buf, a.args[i].Show()...)
	}
	return buf
}

func (a *Script) Read(data []byte) ([]byte, error) {
	var err error
	var name []byte

	if len(data) < SCRIPT_COUNT_SIZE {
		return []byte{}, errors.New("Script Read: Illegal Length")
	}
	n, _ := d32func(data[:SCRIPT_COUNT_SIZE])
	if name, data, err = dslice(data[SCRIPT_COUNT_SIZE:]); err != nil {
		return []byte{}, fmt.Errorf("ScriptArgument Read: %v", err)
	} else {
		a.name = string(name)
	}
	if uint32(len(a.args)) < n {
		a.args = make([]*ScriptArgument, n)
	}
	for i := uint32(0); i < n; i++ {
		if a.args[i] == nil {
			a.args[i] = new(ScriptArgument)
		}
		if data, err = a.args[i].Read(data); err != nil {
			return []byte{}, fmt.Errorf("ScriptArgument Read: %v", err)
		}
	}
	return data, nil
}

func (a *Script) Name() string {
	return a.name
}

func (a *Script) Arguments() []*ScriptArgument {
	return a.args
}

func (a *ScriptArgument) Value() interface{} {
	return a.value
}
