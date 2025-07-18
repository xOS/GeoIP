package http

import (
	"net/http"
	"sync"
)

type appError struct {
	Error       error
	Message     string
	Code        int
	ContentType string
}

// 错误对象池，减少内存分配
var errorPool = sync.Pool{
	New: func() interface{} {
		return &appError{}
	},
}

// getError 从对象池获取错误对象
func getError() *appError {
	return errorPool.Get().(*appError)
}

// putError 将错误对象放回对象池
func putError(e *appError) {
	*e = appError{} // 清空
	errorPool.Put(e)
}

func internalServerError(err error) *appError {
	e := getError()
	e.Error = err
	e.Message = "Internal server error"
	e.Code = http.StatusInternalServerError
	return e
}

func notFound(err error) *appError {
	e := getError()
	e.Error = err
	e.Code = http.StatusNotFound
	return e
}

func badRequest(err error) *appError {
	e := getError()
	e.Error = err
	e.Code = http.StatusBadRequest
	return e
}

func (e *appError) AsJSON() *appError {
	e.ContentType = jsonMediaType
	return e
}

func (e *appError) WithMessage(message string) *appError {
	e.Message = message
	return e
}

func (e *appError) IsJSON() bool {
	return e.ContentType == jsonMediaType
}
