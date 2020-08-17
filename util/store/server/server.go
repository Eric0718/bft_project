package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"kortho/util/store"
	"net"
	"net/textproto"
)

func New(port int, db store.DB) Server {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		return nil
	}
	return &server{db, lis, make(chan struct{})}
}

func (s *server) Run() {
	for {
		select {
		case <-s.ch:
			s.ch <- struct{}{}
			return
		default:
			conn, err := s.lis.Accept()
			if err != nil {
				continue
			}
			go s.serve(conn)
		}
	}
}

func (s *server) Stop() {
	s.ch <- struct{}{}
	<-s.ch
	close(s.ch)
	return
}

func (s *server) serve(conn net.Conn) {
	defer conn.Close()
	for {
		resp := &response{textproto.NewWriter(bufio.NewWriter(conn))}
		req, err := readRequest(bufio.NewReader(conn))
		if err != nil {
			if err != io.EOF {
				resp.respError(errors.New("illegal request"))
			}
			break
		}
		f, ok := dealRegister[string(req.args[0])]
		if !ok {
			resp.respError(fmt.Errorf("unknown command '%s'", string(req.args[0])))
			return
		}
		f(s.db, resp, req.args[1:])
	}
}
