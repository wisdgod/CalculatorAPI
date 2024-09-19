#ifndef CALCULATOR_H
#define CALCULATOR_H

#include <mpfr.h>
#include <stdlib.h>

#ifdef _WIN32
  #define CALC_EXPORT __declspec(dllexport)
#else
  #define CALC_EXPORT __attribute__((visibility("default")))
#endif

#ifdef __cplusplus
extern "C" {
#endif

// 错误码定义
typedef enum {
  CALC_SUCCESS = 0,
  CALC_ERROR_INVALID_INPUT = -1,
  CALC_ERROR_PARSING_FAILED = -2,
  CALC_ERROR_EVALUATION_FAILED = -3,
  CALC_ERROR_BUFFER_TOO_SMALL = -4,
  CALC_ERROR_UNKNOWN = -99
} CalcErrorCode;

// 简化的接口函数
CALC_EXPORT void* create_calculator();
CALC_EXPORT void destroy_calculator(void* calc);
CALC_EXPORT CalcErrorCode calculate_expression(void* calc, const char* input, char* result, size_t size, char* error_msg, size_t error_msg_size);

#ifdef __cplusplus
}
#endif

#endif // CALCULATOR_H