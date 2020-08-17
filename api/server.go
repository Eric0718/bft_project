package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"

	"kortho/logger"

	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

//Server http服务，包含端口信息和路由信息
type Server struct {
	port string
	fasthttprouter.Router
}

//Run 运行http的service
func (s *Server) Run() {

	s.GET("/block", s.GetBlockHandler)
	s.GET("/nonce", s.GetNonceHandler)
	s.GET("/height", s.GetHeightHandler)
	s.GET("/balance", s.GetBalanceHandler)
	s.GET("/transaction", s.GetTransactionHandler)

	if err := fasthttp.ListenAndServe(s.port, s.Handler); err != nil {
		logger.Error("failed to listen port", zap.Error(err), zap.String("port", s.port))
		os.Exit(-1)
	}
}

//GetNonceHandler 获取address的nonce
func (s *Server) GetNonceHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")
	var result resultInfo
	defer func() {
		jsbyte, _ := json.Marshal(result)
		ctx.Write(jsbyte)
	}()
	address := ctx.QueryArgs().Peek("address")
	if len(address) == 0 {
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}

	nonce, err := blockChian.GetNonce(address)
	if err != nil {
		logger.Error("failee to get nonce", zap.Error(err), zap.String("address", string(address)))
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}
	result.Code = successCode
	result.Message = OK
	result.Data = nonce
	ctx.Response.SetStatusCode(http.StatusOK)
	return
}

//GetBalanceHandler 获取address对应余额
func (s *Server) GetBalanceHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")
	var result resultInfo
	defer func() {
		jsbyte, _ := json.Marshal(result)
		ctx.Write(jsbyte)
	}()

	address := ctx.QueryArgs().Peek("address")
	if len(address) == 0 {
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}

	balance, err := blockChian.GetBalance(address)
	if err != nil {
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}

	result.Code = successCode
	result.Message = OK
	result.Data = balance
	ctx.Response.SetStatusCode(http.StatusOK)
	return
}

//GetBlockHandler 获取块数据，输入number参数获取对应的块，否则获取page和size限定的多个块数据
func (s *Server) GetBlockHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")
	var result resultInfo
	defer func() {
		jsbyte, _ := json.Marshal(result)
		ctx.Write(jsbyte)
	}()

	args := ctx.QueryArgs()
	number, numErr := args.GetUint("number")
	page, pageErr := args.GetUint("page")
	size, sizeErr := args.GetUint("size")
	if numErr != nil && (pageErr != nil || sizeErr != nil) {
		logger.Error("Failed to get parameters", zap.Int("number", number), zap.Errors("errors", []error{pageErr, sizeErr}))
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}

	var viewBlocks []Block
	if number > 0 {
		block, err := blockChian.GetBlockByHeight(uint64(number))
		if err != nil {
			logger.Error("Failed to get blockHeight", zap.Error(err), zap.Int("number", number))
			result.Code = failedCode
			result.Message = ErrParameters
			ctx.Response.SetStatusCode(http.StatusBadRequest)
			return
		}
		viewBlocks = append(viewBlocks, changeBlock(block))
	} else if pageErr == nil && sizeErr == nil {
		maxHeight, err := blockChian.GetMaxBlockHeight()
		if err != nil {
			logger.Error("Failed to get maxHeight", zap.Error(err))
			result.Code = failedCode
			result.Message = ErrParameters
			ctx.Response.SetStatusCode(http.StatusBadRequest)
			return
		}
		var start, end uint64
		start = maxHeight - uint64((page-1)*size)
		if maxHeight < uint64(page*size) {
			end = 0
		} else {
			end = maxHeight - uint64(page*size)
		}

		if start < 0 {
			logger.Error("Parameters error", zap.Uint64("max height", maxHeight), zap.Int("page", page), zap.Int("size", size))
			result.Code = failedCode
			result.Message = ErrParameters
			ctx.Response.SetStatusCode(http.StatusBadRequest)
			return
		}

		for ; start > end; start-- {
			block, err := blockChian.GetBlockByHeight(start)
			if err != nil {
				logger.Error("Failed to get block", zap.Error(err), zap.Uint64("height", start))
				result.Code = failedCode
				result.Message = ErrParameters
				ctx.Response.SetStatusCode(http.StatusBadRequest)
				return
			}
			viewBlocks = append(viewBlocks, changeBlock(block))
		}
	} else {
		logger.Error("Parameters error", zap.Int("number", number), zap.Int("page", page), zap.Int("size", size))
		result.Code = failedCode
		result.Message = ErrParameters
		ctx.Response.SetStatusCode(http.StatusBadRequest)
		return
	}

	result.Code = successCode
	result.Message = OK
	result.Data = viewBlocks
	ctx.Response.SetStatusCode(http.StatusOK)
	return
}

// GetHeightHandler 获取块高
func (s *Server) GetHeightHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")
	var result resultInfo
	defer func() {
		bytes, _ := json.Marshal(result)
		ctx.Write(bytes)
	}()

	height, err := blockChian.GetHeight()
	if err != nil {
		result.Code = -1
		result.Message = "内部错误"
	}

	result.Code = successCode
	result.Message = OK
	result.Data = height
	return
}

//GetTransactionHandler 获取hash对应的交易数据，或者获取对应address的分页交易数据
func (s *Server) GetTransactionHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")
	var result resultInfo
	defer func() {
		jsbyte, _ := json.Marshal(result)
		ctx.Write(jsbyte)
	}()

	args := ctx.QueryArgs()
	address := args.Peek("address")
	hash := args.Peek("hash")
	page, _ := args.GetUint("page")
	size, _ := args.GetUint("size")

	start := (page - 1) * size
	end := start + size - 1

	var viewTxs []Transaction
	if len(address) != 0 {
		if start > end {
			logger.Error("parameter error", zap.Int("page", page), zap.Int("size", size))
			result.Code = -1
			result.Message = "failed"
			return
		}

		txs, err := blockChian.GetTransactionByAddr(address, int64(start), int64(end))
		if err != nil {
			logger.Error("Failed to get transactions", zap.Error(err), zap.String("address", string(address)),
				zap.Int("start", start), zap.Int("end", end))
			result.Code = -1
			result.Message = "failed"
			return
		}
		logger.Info("transaction length", zap.Int("len", len(txs)))
		for _, tx := range txs {
			viewTxs = append(viewTxs, changeTransaction(tx))
		}
		result.Data = viewTxs
	} else if len(hash) != 0 {
		hashBytes, _ := hex.DecodeString(string(hash))
		tx, err := blockChian.GetTransactionByHash(hashBytes)
		if err != nil {
			logger.Error("Failed to get transactions", zap.Error(err), zap.String("hash", string(hash)))
			result.Code = failedCode
			result.Message = ErrParameters
			return
		}
		result.Data = append(viewTxs, changeTransaction(tx))
	} else {
		txs, err := blockChian.GetTransactions(int64(start), int64(end))
		if err != nil {
			logger.Error("Failed to get transactions", zap.Error(err), zap.Int("start", start), zap.Int("end", end))
			result.Code = failedCode
			result.Message = ErrParameters
			return
		}

		for _, tx := range txs {
			viewTxs = append(viewTxs, changeTransaction(tx))
		}
		result.Data = viewTxs
	}
	result.Code = successCode
	result.Message = OK
	ctx.Response.SetStatusCode(http.StatusOK)
	return
}
