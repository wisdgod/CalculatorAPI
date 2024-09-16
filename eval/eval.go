package eval

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
)

// Value 表示计算器中的值，支持高精度模式
type Value struct {
	Real  *big.Float
	Imag  *big.Float
	Str   string
	IsStr bool
	IsInt bool
}

// Eval 结构体用于遍历和求值抽象语法树（AST）
type Eval struct {
	info          types.Info
	vars          map[string]Value
	funcs         map[string]func([]Value, bool) (Value, error)
	result        Value
	hasReturn     bool
	highPrecision bool
}

// NewEval 创建一个新的 Eval 实例
func NewEval(highPrecision bool) *Eval {
	e := &Eval{
		info: types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		},
		vars:          make(map[string]Value),
		funcs:         make(map[string]func([]Value, bool) (Value, error)),
		highPrecision: highPrecision,
	}
	e.registerBuiltinFuncs()
	return e
}

// registerBuiltinFuncs 注册内置函数
func (e *Eval) registerBuiltinFuncs() {
	e.funcs = map[string]func([]Value, bool) (Value, error){
		"sqrt": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("sqrt 函数需要一个参数")
			}
			if highPrecision {
				result := new(big.Float).Sqrt(args[0].Real)
				return Value{Real: result}, nil
			}
			return Value{Real: new(big.Float).SetFloat64(math.Sqrt(floatValue(args[0].Real)))}, nil
		},
		"pow": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 2 {
				return Value{}, fmt.Errorf("pow 函数需要两个参数")
			}
			result := math.Pow(floatValue(args[0].Real), floatValue(args[1].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"rand": func(args []Value, highPrecision bool) (Value, error) {
			return Value{Real: new(big.Float).SetFloat64(rand.Float64())}, nil
		},
		"sin": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("sin 函数需要一个参数")
			}
			result := math.Sin(floatValue(args[0].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"cos": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("cos 函数需要一个参数")
			}
			result := math.Cos(floatValue(args[0].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"tan": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("tan 函数需要一个参数")
			}
			result := math.Tan(floatValue(args[0].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"log": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("log 函数需要一个参数")
			}
			result := math.Log(floatValue(args[0].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"exp": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("exp 函数需要一个参数")
			}
			result := math.Exp(floatValue(args[0].Real))
			return Value{Real: new(big.Float).SetFloat64(result)}, nil
		},
		"complex": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 2 {
				return Value{}, fmt.Errorf("complex 函数需要两个参数")
			}
			return Value{Real: args[0].Real, Imag: args[1].Real}, nil
		},
		"re": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("re 函数需要一个参数")
			}
			return Value{Real: args[0].Real}, nil
		},
		"im": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("im 函数需要一个参数")
			}
			return Value{Real: args[0].Imag}, nil
		},
		"string": func(args []Value, highPrecision bool) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("string 函数需要一个参数")
			}
			if !args[0].IsStr {
				return Value{}, fmt.Errorf("string 函数的参数必须是字符串")
			}

			var result int64
			for _, r := range args[0].Str {
				result = result*31 + int64(r)
			}
			return Value{Real: new(big.Float).SetInt64(result)}, nil
		},
	}
}

// Visit 方法是访问 AST 节点的函数，根据节点类型来执行不同的操作。
func (e *Eval) Visit(node ast.Node) ast.Visitor {
	if node == nil || e.hasReturn {
		return nil
	}

	switch n := node.(type) {
	case *ast.ReturnStmt:
		if len(n.Results) > 0 {
			result, err := e.evalExpr(n.Results[0])
			if err != nil {
				fmt.Println("Error:", err)
				return nil
			}
			e.result = result
			e.hasReturn = true
		}
		return nil
	case *ast.ExprStmt:
		result, err := e.evalExpr(n.X)
		if err != nil {
			fmt.Println("Error:", err)
			return nil
		}
		e.result = result
	case *ast.AssignStmt:
		if err := e.evalAssign(n); err != nil {
			fmt.Println("Error:", err)
			return nil
		}
		// 如果是临时变量赋值，更新结果
		if len(n.Lhs) == 1 {
			if ident, ok := n.Lhs[0].(*ast.Ident); ok && strings.HasPrefix(ident.Name, "__temp") {
				e.result = e.vars[ident.Name]
			}
		}
	}
	return e
}

// floatValue 辅助函数，用于从 big.Float 获取 float64 值
func floatValue(f *big.Float) float64 {
	v, _ := f.Float64()
	return v
}

// evalExpr 评估一个表达式并返回结果
func (e *Eval) evalExpr(expr ast.Expr) (Value, error) {
	switch n := expr.(type) {
	case *ast.BasicLit:
		switch n.Kind {
		case token.INT, token.FLOAT:
			if e.highPrecision {
				val, _, err := new(big.Float).Parse(n.Value, 10)
				return Value{Real: val}, err
			}
			val, err := strconv.ParseFloat(n.Value, 64)
			return Value{Real: new(big.Float).SetFloat64(val)}, err
		case token.STRING:
			return Value{Str: n.Value[1 : len(n.Value)-1], IsStr: true}, nil
		default:
			return Value{}, fmt.Errorf("不支持的字面量类型: %v", n.Kind)
		}
	case *ast.Ident:
		if val, ok := e.vars[n.Name]; ok {
			return val, nil
		}
		return Value{}, fmt.Errorf("未定义的变量: %s", n.Name)
	case *ast.ParenExpr:
		return e.evalExpr(n.X)
	case *ast.BinaryExpr:
		return e.evalBinaryExpr(n)
	case *ast.CallExpr:
		return e.evalFuncCall(n)
	default:
		return Value{}, fmt.Errorf("不支持的表达式类型: %T", expr)
	}
}

// evalBinaryExpr 处理二元操作
func (e *Eval) evalBinaryExpr(n *ast.BinaryExpr) (Value, error) {
	left, err := e.evalExpr(n.X)
	if err != nil {
		return Value{}, err
	}
	right, err := e.evalExpr(n.Y)
	if err != nil {
		return Value{}, err
	}

	if left.IsStr || right.IsStr {
		return e.evalStringBinaryExpr(left, right, n.Op)
	}

	result := new(big.Float).SetPrec(256) // 设置高精度
	switch n.Op {
	case token.ADD:
		result.Add(left.Real, right.Real)
	case token.SUB:
		result.Sub(left.Real, right.Real)
	case token.MUL:
		result.Mul(left.Real, right.Real)
	case token.QUO:
		if right.Real.Sign() == 0 {
			return Value{}, fmt.Errorf("除数不能为零")
		}
		result.Quo(left.Real, right.Real)
	case token.REM:
		if e.highPrecision {
			// 高精度模式下的取模运算
			quotient := new(big.Float).Quo(left.Real, right.Real)
			intQuotient := new(big.Float).SetFloat64(math.Floor(floatValue(quotient)))
			temp := new(big.Float).Mul(intQuotient, right.Real)
			result.Sub(left.Real, temp)
		} else {
			leftFloat, _ := left.Real.Float64()
			rightFloat, _ := right.Real.Float64()
			result.SetFloat64(math.Mod(leftFloat, rightFloat))
		}
	case token.XOR: // 使用 XOR 代替幂运算
		leftFloat, _ := left.Real.Float64()
		rightFloat, _ := right.Real.Float64()
		result.SetFloat64(math.Pow(leftFloat, rightFloat))
	default:
		return Value{}, fmt.Errorf("不支持的操作符: %v", n.Op)
	}

	// 设置精度和舍入模式，而不是使用不存在的 Round 方法
	result.SetPrec(53).SetMode(big.ToNearestEven)

	return Value{Real: result}, nil
}

// evalStringBinaryExpr 处理字符串的二元操作。
func (e *Eval) evalStringBinaryExpr(left, right Value, op token.Token) (Value, error) {
	switch op {
	case token.ADD:
		return Value{Str: left.Str + right.Str, IsStr: true}, nil
	// 可以添加其他字符串操作
	default:
		return Value{}, fmt.Errorf("不支持的字符串操作: %v", op)
	}
}

// evalFuncCall 处理函数调用
func (e *Eval) evalFuncCall(call *ast.CallExpr) (Value, error) {
	funcName, ok := call.Fun.(*ast.Ident)
	if !ok {
		return Value{}, fmt.Errorf("无效的函数调用")
	}

	fn, ok := e.funcs[funcName.Name]
	if !ok {
		return Value{}, fmt.Errorf("未知函数: %s", funcName.Name)
	}

	args := make([]Value, len(call.Args))
	for i, arg := range call.Args {
		val, err := e.evalExpr(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}

	return fn(args, e.highPrecision)
}

// evalAssign 处理赋值语句。
func (e *Eval) evalAssign(stmt *ast.AssignStmt) error {
	if len(stmt.Lhs) != len(stmt.Rhs) {
		return fmt.Errorf("赋值语句左右两边的数量不匹配")
	}

	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			return fmt.Errorf("赋值语句左边必须是标识符")
		}

		value, err := e.evalExpr(stmt.Rhs[i])
		if err != nil {
			return err
		}

		e.vars[ident.Name] = value
	}

	return nil
}

// parseString 解析包含转义字符的字符串
func parseString(s string) string {
	var result strings.Builder
	escape := false
	for _, ch := range s {
		if escape {
			switch ch {
			case 'n':
				result.WriteRune('\n')
			case 't':
				result.WriteRune('\t')
			case '\\':
				result.WriteRune('\\')
			case '(':
				result.WriteRune('(')
			case ')':
				result.WriteRune(')')
			default:
				result.WriteRune('\\')
				result.WriteRune(ch)
			}
			escape = false
		} else if ch == '\\' {
			escape = true
		} else {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// preprocessString 函数预处理输入的string()
func preprocessString(expr string) string {
	// 使用正则表达式来匹配 string() 函数调用，包括嵌套的括号和转义字符
	re := regexp.MustCompile(`string\(((?:[^()\\]|\\.|\((?:[^()\\]|\\.)*\))*)\)`)
	return re.ReplaceAllStringFunc(expr, func(match string) string {
		// 提取括号内的内容
		inner := match[7 : len(match)-1]

		// 检查是否为反引号字符串
		if strings.HasPrefix(inner, "`") && strings.HasSuffix(inner, "`") {
			// 去除反引号并直接返回内容
			return fmt.Sprintf("string(%s)", strconv.Quote(inner[1:len(inner)-1]))
		}

		// 如果内容已经是字符串（被双引号包围），则不做改变
		if strings.HasPrefix(inner, "\"") && strings.HasSuffix(inner, "\"") {
			return match
		}

		// 处理转义字符
		var processedInner strings.Builder
		for i := 0; i < len(inner); i++ {
			if i < len(inner)-1 && inner[i] == '\\' {
				switch inner[i+1] {
				case ')', '(', '\\':
					processedInner.WriteByte(inner[i+1])
					i++ // 跳过下一个字符
				default:
					processedInner.WriteString("\\")
					processedInner.WriteByte(inner[i+1])
					i++ // 跳过下一个字符
				}
			} else {
				processedInner.WriteByte(inner[i])
			}
		}

		// 返回处理后的内容，使用 strconv.Quote 确保正确的转义
		return fmt.Sprintf("string(%s)", strconv.Quote(processedInner.String()))
	})
}

// Evaluate 函数接收表达式字符串和高精度模式标志，并返回求值结果
func Evaluate(expr string, highPrecision bool) (string, error) {
	// 预处理输入，处理无引号字符串
	expr = preprocessString(expr)

	statements := strings.Split(expr, ";")

	fs := token.NewFileSet()
	evaluator := NewEval(highPrecision)

	// 创建内置函数的虚拟声明
	var builtinDecls []string
	for funcName := range evaluator.funcs {
		builtinDecls = append(builtinDecls, fmt.Sprintf("var %s func(...interface{}) interface{}", funcName))
	}
	builtinDeclStr := strings.Join(builtinDecls, "\n")

	var lastResult Value
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// 自动转换单独的表达式为赋值语句
		if !strings.Contains(stmt, "=") && !strings.HasPrefix(stmt, "return") {
			stmt = fmt.Sprintf("__temp%d := %s;_ = __temp%d", i, stmt, i)
		}

		// 将语句包装在函数体中，并包含内置函数声明
		wrappedStmt := fmt.Sprintf("package main\n\n%s\n\nfunc main() {\n\t%s\n}", builtinDeclStr, stmt)

		f, err := parser.ParseFile(fs, "", wrappedStmt, parser.ParseComments)
		if err != nil {
			return "", fmt.Errorf("parse error in statement '%s': %v", stmt, err)
		}

		// 创建一个新的类型检查器
		conf := types.Config{Importer: nil}
		info := &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		}

		// 执行类型检查
		_, err = conf.Check("main", fs, []*ast.File{f}, info)
		if err != nil {
			return "", fmt.Errorf("type check error in statement '%s': %v", stmt, err)
		}

		// 更新 evaluator 的 info
		evaluator.info = *info

		// 重置 hasReturn 标志
		evaluator.hasReturn = false

		// 遍历 AST
		ast.Walk(evaluator, f)

		// 如果是返回语句，直接返回结果
		if evaluator.hasReturn {
			return formatResult(evaluator.result, highPrecision), nil
		}

		// 否则，更新最后的结果
		lastResult = evaluator.result
	}

	// 返回最后一个表达式的结果
	return formatResult(lastResult, highPrecision), nil
}

// formatResult 格式化结果为字符串
func formatResult(result Value, highPrecision bool) string {
	if result.IsStr {
		return parseString(result.Str)
	} else {
		if highPrecision {
			// 使用更高的精度，并设置适当的舍入模式
			rounded := new(big.Float).SetPrec(256).Set(result.Real)
			rounded.SetMode(big.ToNearestEven)
			return rounded.Text('f', 50) // 使用50位精度
		} else {
			f, _ := result.Real.Float64()
			return fmt.Sprintf("%v", f)
		}
	}
}
