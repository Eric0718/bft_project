package parser

const (
	SUB_OFF  = 8
	TYP_MASK = 0xFF
	SUB_MASK = 0xFF00
)

// 8 ~ 16 bit, sub type of O_CONST
const (
	O_INT    = 0x01
	O_STRING = 0x02
)

const (
	O_CALL  = 0x01
	O_NAME  = 0x02
	O_CONST = 0x03
	O_ELIST = 0x04
)

const (
	UnDefined              = 0x00
	FunctionCALL           = 0x01
	ArgumentExpressionList = 0x02
	PrimaryExpression      = 0x04
)

type scriptParser struct {
	wt         *scriptLex
	ns         []*scriptNode
	rollBuffer *scriptParserRollBuffer
}

type scriptParserRollBuffer struct {
	cnt    int           // 回滚数目，最大ROLLSIZE
	buffer []*scriptWord // 回滚缓冲区
}

type scriptNode struct {
	op    int
	left  *scriptNode
	right *scriptNode
	value interface{}
}

type scriptParserFunc (func(*scriptParser) (*scriptNode, error))

var parserRegistry map[int]scriptParserFunc

var firstList = []int{
	FunctionCALL,
	PrimaryExpression | ArgumentExpressionList,
	PrimaryExpression | ArgumentExpressionList,
}
