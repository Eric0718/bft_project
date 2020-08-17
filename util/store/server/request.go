package server

import (
	"bufio"
	"bytes"
	"errors"
	"net/textproto"
	"strconv"
)

func readRequest(r *bufio.Reader) (*request, error) {
	req := new(request)
	tr := textproto.NewReader(r)
	n, err := readNumber(tr)
	if err != nil {
		return nil, err
	}
	for i := int64(0); i < n; i++ {
		x, err := readNumber(tr)
		if err != nil {
			return nil, err
		}
		arg := make([]byte, x+2)
		if y, _ := r.Read(arg); x+2 != int64(y) {
			return nil, errors.New("Argument Illegal Length")
		}
		if i == 0 {
			arg = bytes.ToUpper(arg)
		}
		req.args = append(req.args, arg[:len(arg)-2])
	}
	return req, nil
}

func readNumber(r *textproto.Reader) (int64, error) {
	s, err := r.ReadLine()
	if err != nil {
		return -1, err
	}
	return strconv.ParseInt(s[1:], 0, 64)
}
