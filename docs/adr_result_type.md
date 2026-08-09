# ADR 001: 统一 Result<T, E> 类型在多后端语言中的生成策略

## 状态
Accepted

## 背景
xiaoqinli transpiler 需要支持健壮的错误处理语义。目前在 AST 中支持了 `Result` 这种泛型结构（例如 `Result<Int, String>`），但对于不同的语言后端，错误处理的惯例（Idiom）和内置结构大相径庭（例如 Go 习惯用多返回值 `(T, error)`，Rust 习惯用 `Result<T, E>`，而 TypeScript/Python 主要依靠 Exception）。

为了确保同一份包含错误处理的 `.xql` 代码能够被转译至各主力后端并生成语义等价、可运行、可编译的代码，必须统一其转译落地策略。

## 决策

### 1. Rust 后端
- **转译策略**：直接生成为 Rust 的原生泛型结构 `Result<T, E>`。
- **值构造**：构造成功的值生成为 `Ok(value)`，构造错误的值生成为 `Err(error)`。

### 2. Go 后端
- **转译策略**：将 `Result<T, E>` 返回类型映射为 Go 最天然的二元多返回值 `(T, error)`。
- **值构造与返回**：
  - 如果是成功返回，则将其转译为 `return value, nil`。
  - 如果是失败返回，则将其转译为 `return zero_value, fmt.Errorf(...)`。
- **调用接收**：如果被调用的函数返回 `Result`，在 Go 生成逻辑中需要通过 `res, err := func()` 二元解包来接收。

### 3. TypeScript 后端
- **转译策略**：在 TypeScript 后端中引入一个轻量级的包装类 `Result<T, E>` 以完美保留静态类型与流控制，而不必使用不透明的 Exception（异常）。
- **Result 包装类定义**：
  ```typescript
  class Result<T, E> {
      private constructor(
          private readonly _ok: T | null,
          private readonly _err: E | null,
          public readonly isOk: boolean
      ) {}
      static ok<T, E>(val: T): Result<T, E> {
          return new Result<T, E>(val, null, true);
      }
      static err<T, E>(err: E): Result<T, E> {
          return new Result<T, E>(null, err, false);
      }
      unwrap(): T {
          if (!this.isOk) throw new Error("Called unwrap on Err Result");
          return this._ok!;
      }
      unwrapErr(): E {
          if (this.isOk) throw new Error("Called unwrapErr on Ok Result");
          return this._err!;
      }
  }
  ```
- **生成策略**：在 codegen 时，如果使用了 `Result<T, E>` 返回或相关字面量，自动在生成文件的头部输出该 `Result` 类的定义辅助函数，构造成功值生成为 `Result.ok(val)`，错误值生成为 `Result.err(err)`。

### 4. Python 后端
- **转译策略**：与 TypeScript 类似，在生成的 Python 代码中附带一个 `Result` 类定义：
  ```python
  class Result:
      def __init__(self, ok_val=None, err_val=None, is_ok=True):
          self._ok = ok_val
          self._err = err_val
          self.is_ok = is_ok
      @staticmethod
      def ok(val):
          return Result(ok_val=val, is_ok=True)
      @staticmethod
      def err(err_val):
          return Result(err_val=err_val, is_ok=False)
      def unwrap(self):
          if not self.is_ok:
              raise Exception("Called unwrap on Err Result")
          return self._ok
      def unwrap_err(self):
          if self.is_ok:
              raise Exception("Called unwrap_err on Ok Result")
          return self._err
  ```

## 后果
- 保证了在所有主流目标语言后端上的**语义一致性**与**类型安全性**。
- 极大地降低了生成端对 Exception 处理的负担，保留了 AST-First 中纯净的流分析树。

## 后端约束：Kotlin / Android 的名字遮蔽

Kotlin 标准库自带单参数的 `kotlin.Result<out T>`，且它在每个文件的默认导入里。
XQL 的 `Result<T, E>` 是双参数的，名字撞上后编译器报 "One type argument expected"。

因此 `kotlin` 与 `android` 后端必须把自己的 `Result` 声明发到生成文件所在的包
的顶层——Kotlin 的解析顺序里，同包声明优先于默认导入，这样才能遮蔽掉标准库那个。
两个后端共用 `codegen.kotlinResultClass` 一份定义，避免各写一份后漂移。
