package parser

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

func (wr *scriptLexRollBuffer) putc(c int) int {
	for i := ROLLSIZE - 1; i < 0; i-- {
		wr.buffer[i] = wr.buffer[i-1]
	}
	wr.buffer[0] = c
	return c
}

// 试图从回滚字符缓冲区获取字符，成功返回字符，失败返回-2
func (wr *scriptLexRollBuffer) getc() int {
	if wr.cnt > 0 {
		wr.cnt--
		return wr.buffer[wr.cnt]
	}
	return RET
}

// 回滚字符
func (wr *scriptLexRollBuffer) ungetc() {
	if wr.cnt < ROLLSIZE {
		wr.cnt++
	}
}

func (wt *scriptLex) getc() int {
	if c := wt.rollBuffer.getc(); c != RET {
		return c
	}
	if wt.buffer.pos == wt.buffer.lim {
		if n, _ := wt.fd.Read(wt.buffer.buffer); n == 0 {
			return wt.rollBuffer.putc(EOF)
		} else {
			wt.buffer.pos = 0
			wt.buffer.lim = n
		}
	}
	c := wt.rollBuffer.putc(int(wt.buffer.buffer[wt.buffer.pos]))
	wt.buffer.pos++
	return c
}

func (wt *scriptLex) ungetc() {
	wt.rollBuffer.ungetc()
}

// 跳过空行
func (wt *scriptLex) skipNewline() {
	for {
		if c := wt.getc(); isEof(c) || !isNewline(c) {
			wt.ungetc()
			return
		}
	}
}

func (wt *scriptLex) getWord(a int) int {
	word := []byte{byte(a)}
	for {
		if c := wt.getc(); isEof(c) || !isAlnum(c) {
			wt.ungetc()
			wt.curr.typ = IDENTIFIER
			wt.curr.name = string(word)
			return wt.curr.typ
		} else {
			word = append(word, byte(c))
		}
	}
}

// 解析字符常量或者字符串常量
func (wt *scriptLex) getLiteral(a, typ int) int {
	s := []byte{'`', byte(a)}
	for {
		c := wt.getc()
		switch {
		case isEof(c), isNewline(c):
			wt.err = errors.New("Fate: Illegal Literal Constant Format")
			return ERR
		case a == c:
			s = append(s, []byte{byte(c), '`'}...)
			if v, err := strconv.Unquote(string(s)); err == nil {
				if wt.curr.typ = typ; typ == STRING_CONSTANT {
					wt.curr.value = v[1 : len(v)-1]
				} else {
					wt.curr.value = byte(v[1])
				}
				return typ
			} else {
				wt.err = fmt.Errorf("Fate: Illegal Literal Constant Format: %v", err)
				return ERR
			}
		default:
			s = append(s, byte(c))
		}
	}
}

// 解析数字
func (wt *scriptLex) getDigit(a int) int {
	var c int

	word := []byte{}
	typ := INT_CONSTANT
	word = append(word, byte(a))
	if a == '0' {
		c = wt.getc()
		switch {
		case isEof(c):
			goto out
		case c != 'x' && c != 'X' && !isDigit(c):
			goto out
		}
		wt.ungetc()
	}
	for {
		c = wt.getc()
		switch {
		case isEof(c):
			goto out
		case !isXdigit(c):
			goto out
		default:
			word = append(word, byte(c))
		}
	}
out:
	wt.ungetc()
	switch wt.curr.typ = typ; typ {
	case INT_CONSTANT:
		v, err := strconv.ParseUint(string(word), 0, 64)
		if err != nil {
			wt.err = err
			return ERR
		}
		wt.curr.value = v
	}
	return typ
}

func (wt *scriptLex) symbol() int {
	var c int

	for {
		c = wt.getc()
		switch {
		case isEof(c):
			return c
		case isSpace(c):
		case isNewline(c):
			wt.skipNewline()
		case isAlpha(c):
			return wt.getWord(c)
		case isDigit(c):
			return wt.getDigit(c)
		case isSingleQuotation(c), isDoubleQuotation(c):
			return wt.getLiteral(c, STRING_CONSTANT)
		default:
			wt.curr.typ = charTypeList[c&_LIM]
			return wt.curr.typ
		}
	}
}

func NewLex(fd io.Reader) *scriptLex {
	return &scriptLex{
		fd:   fd,
		curr: &scriptWord{},
		buffer: &scriptLexBuffer{
			buffer: make([]byte, BUFFSIZE),
		},
		rollBuffer: &scriptLexRollBuffer{
			buffer: make([]int, ROLLSIZE),
		},
	}
}
