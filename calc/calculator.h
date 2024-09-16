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

// 简化的接口函数
CALC_EXPORT void* create_calculator();
CALC_EXPORT void destroy_calculator(void* calc);
CALC_EXPORT int calculate_expression(void* calc, const char* input, char* result, size_t size);

#ifdef __cplusplus
}
#endif

#endif // CALCULATOR_H