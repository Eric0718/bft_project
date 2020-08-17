package server

func (resp *response) respError(err error) {
	resp.w.PrintfLine("-Error %v", err)
}

func (resp *response) respInteger(v int) {
	resp.w.PrintfLine(":%v", v)
}

func (resp *response) respSimple(s string) {
	resp.w.PrintfLine("+%s", s)
}

func (resp *response) respBulk(s string) {
	switch len(s) {
	case 0:
		resp.w.PrintfLine("$-1")
	default:
		resp.w.PrintfLine("$%v", len(s))
		resp.w.PrintfLine("%s", s)
	}
}

func (resp *response) respArray(xs [][]byte) {
	resp.w.PrintfLine("*%v", len(xs))
	for i, j := 0, len(xs); i < j; i++ {
		switch len(xs[i]) {
		case 0:
			resp.w.PrintfLine("$-1")
		default:
			resp.w.PrintfLine("$%v", len(xs[i]))
			resp.w.PrintfLine("%s", string(xs[i]))
		}
	}
}
