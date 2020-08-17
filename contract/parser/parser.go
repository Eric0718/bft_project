package parser

import (
	"errors"
	"io"
)

func init() {
	parserRegistry = map[int]scriptParserFunc{
		FunctionCALL:           parser0,
		ArgumentExpressionList: parser1,
		PrimaryExpression:      parser2,
		UnDefined:              parserAbort,
	}
}

func (wr *scriptParserRollBuffer) putw(w *scriptWord) *scriptWord {
	for i := ROLLSIZE - 1; i < 0; i-- {
		wr.buffer[i] = wr.buffer[i-1]
	}
	wr.buffer[0] = w
	return w
}

func (wr *scriptParserRollBuffer) getw() *scriptWord {
	if wr.cnt > 0 {
		wr.cnt--
		return wr.buffer[wr.cnt]
	}
	return nil
}

func (wr *scriptParserRollBuffer) ungetw() {
	if wr.cnt < ROLLSIZE {
		wr.cnt++
	}
}

func (wp *scriptParser) ungetw() {
	wp.rollBuffer.ungetw()
}

func (wp *scriptParser) getw() *scriptWord {
	if w := wp.rollBuffer.getw(); w != nil {
		return w
	}
	switch o := wp.wt.symbol(); o {
	case EOF:
		return nil
	case ERR:
		return nil
	}
	return wp.rollBuffer.putw(&scriptWord{
		typ:   wp.wt.curr.typ,
		name:  wp.wt.curr.name,
		value: wp.wt.curr.value,
	})
}

func (wp *scriptParser) peek() int {
	if w := wp.getw(); w != nil {
		wp.ungetw()
		return w.typ
	}
	return EOF
}

func (wp *scriptParser) detect0(typ int) bool {
	return wp.peek() == typ
}

func (wp *scriptParser) detect1(grm int) bool {
	if typ := wp.peek(); typ == EOF {
		return false
	} else {
		return firstList[typ]&grm != 0
	}
}

// Abort
func parserAbort(wp *scriptParser) (*scriptNode, error) {
	return nil, errors.New("Abort")
}

// FunctionCALL
func parser0(wp *scriptParser) (*scriptNode, error) {
	var err error
	var n1, n2 *scriptNode

	switch {
	case wp.detect0(IDENTIFIER):
		name := wp.getw().name
		n1 = &scriptNode{
			value: name,
			op:    O_NAME,
		}
	default:
		return nil, errors.New("Expect Function Name")
	}
	if wp.detect0(EOF) {
		wp.getw() // skip EOF
		return &scriptNode{
			left: n1,
			op:   O_CALL,
		}, nil
	}
	if n2, err = parserRegistry[ArgumentExpressionList](wp); err != nil {
		return nil, err
	}
	return &scriptNode{
		left:  n1,
		right: n2,
		op:    O_CALL,
	}, nil
}

// ArgumentExpressionList -- left = PrimaryExpression, next = ArgumentExpressionList
func parser1(wp *scriptParser) (*scriptNode, error) {
	var err error
	var n1, n2 *scriptNode

	if n1, err = parserRegistry[PrimaryExpression](wp); err != nil {
		return nil, err
	}
	if wp.detect0(EOF) {
		wp.getw() // skip EOF
		return n1, nil
	}
	if !wp.detect1(PrimaryExpression) {
		return nil, errors.New("Expect PrimaryExpression")
	}
	if n2, err = parserRegistry[ArgumentExpressionList](wp); err != nil {
		return nil, err
	}
	return &scriptNode{
		left:  n1,
		right: n2,
		op:    O_ELIST,
	}, nil

}

// PrimaryExpression
func parser2(wp *scriptParser) (*scriptNode, error) {
	switch {
	case wp.detect0(INT_CONSTANT):
		return &scriptNode{
			value: wp.getw().value,
			op:    O_CONST | O_INT<<SUB_OFF,
		}, nil
	case wp.detect0(STRING_CONSTANT):
		return &scriptNode{
			value: wp.getw().value,
			op:    O_CONST | O_STRING<<SUB_OFF,
		}, nil
	default:
		return nil, errors.New("Expect PrimaryExpression")
	}
}

func NewParser(fd io.Reader) *scriptParser {
	wt := NewLex(fd)
	if wt == nil {
		return nil
	}
	return &scriptParser{
		wt: wt,
		ns: []*scriptNode{},
		rollBuffer: &scriptParserRollBuffer{
			cnt:    0,
			buffer: make([]*scriptWord, ROLLSIZE),
		},
	}
}

func (wp *scriptParser) Parser() error {
	var err error
	var node *scriptNode

	for {
		switch typ := wp.peek(); typ {
		case EOF:
			return nil
		case IDENTIFIER:
			node, err = parserRegistry[FunctionCALL](wp)
		default:
			return errors.New("Expect Function Call")
		}
		if err != nil {
			return err
		}
		wp.ns = append(wp.ns, node)
	}
}
