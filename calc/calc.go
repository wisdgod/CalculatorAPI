package calc

/*
#cgo LDFLAGS: -L. -lcalc_parse -Wl,-rpath,$ORIGIN
#include "calculator.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"math/big"
	"unsafe"
)

const (
	initialBufferSize = 1024 * 64        // 64KB
	maxBufferSize     = 1024 * 1024 * 64 // 64MB
	errorBufferSize   = 1024             // 1KB for error messages
)

// Calculate 评估一个数学表达式并返回 big.Float 类型的结果。
func Calculate(expr string) (*big.Float, error) {
	// 创建一个新的计算器实例
	calc := C.create_calculator()
	if calc == nil {
		return nil, errors.New("创建计算器实例失败")
	}
	defer C.destroy_calculator(calc)

	// 将 Go 字符串转换为 C 字符串
	cExpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cExpr))

	// 使用动态缓冲区大小
	bufSize := initialBufferSize
	var result string
	var resultCode C.CalcErrorCode

	// 创建错误消息缓冲区
	errorBuf := make([]byte, errorBufferSize)
	cErrorBuf := (*C.char)(unsafe.Pointer(&errorBuf[0]))

	for bufSize <= maxBufferSize {
		buf := make([]byte, bufSize)
		cBuf := (*C.char)(unsafe.Pointer(&buf[0]))

		resultCode = C.calculate_expression(calc, cExpr, cBuf, C.size_t(bufSize), cErrorBuf, C.size_t(errorBufferSize))

		if resultCode == C.CALC_SUCCESS {
			result = C.GoString(cBuf)
			break
		} else if resultCode != C.CALC_ERROR_BUFFER_TOO_SMALL {
			return nil, fmt.Errorf("计算失败: %s", C.GoString(cErrorBuf))
		}

		// 增加缓冲区大小并重试
		bufSize *= 2
	}

	if resultCode != C.CALC_SUCCESS {
		return nil, errors.New("结果超出最大缓冲区大小")
	}

	// 将结果字符串解析为 big.Float
	f := new(big.Float)
	_, _, err := f.Parse(result, 99999)
	if err != nil {
		return nil, fmt.Errorf("解析结果失败: %v", err)
	}

	return f, nil
}
