package api

const (
	successCode = 0
	failedCode  = 1
)

const (
	// OK 接口成功返回状态
	OK = "ok"
	// ErrParameters 错误的参数
	ErrParameters = "参数错误"
)

type resultInfo struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
