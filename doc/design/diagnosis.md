# SQLFlow Diagnostics

For a typical SQLFlow method calling link:

``` golang
// pkg/sqlflowserver
func Do(program string, sess *Session) string {
  return workflow.Run(
    couler.GenCode(
      verifier.Verify(
        resolver.ResolveProgram(parser.Parse(program)),
        sess.DB)))
}
```

Any of the above function calls can raise an error, it may be a syntax error on parsing, or verify fault on verifying prediction and training data schema, or the submitter program running error, etc.

 This document issues an SQLFlow Error package `error` to improve the error message maintainability and readability and maintain various error messages in a rigorous approach.

## Error Package

In order to distinguish which package the error occurred, e.g., `parser`, `verify`, we represent an enumerated type `ErrType`. For each `ErrType`, there would be various types errors, for an example, verifying would fault on **attribute value type error** or **attribute does not exist**, we represent an enumerated type variable: `ErrCode`, The `ErrCode` can be the same among different `ErrType`.

``` golang
// ErrType represents the errors in a pacakge
type ErrType int

const (
  pkgVerify ErrType iota
  pkgParser
  ...
)

func (t ErrType) String() string {
  return []string{"verify", "parser", ...}[t]
}


// ErrCode represents a specific error type of a ErrType
type ErrCode int

```

To be compliant with the standard `error` package in Go, we represent the `SQLFlowError` struct which implements `error` interface:

``` golang

type SQLFlowError struct {
  pkg       ErrType
  code      ErrCode
  message   string
}

func (e SQLFlowError) Error() string {
  fmt.Sprintf("%s:%d, %s", e.pkg, e.code, e.message)
}

func (e SQLFlowError) Format(arg ...interface{}) SQLFlowError {
  e.message = fmt.Sprintf(e.message, arg...)
  return e
}

func (p ErrType) New(code ErrCode, message string) SQLFlowError {
  return &SQLFlowError {
    pkg:      p,
    code:     code,
    message:  message
  }
}
```

### An Example of Usage

We want uses can use `error` package as the following code:

``` golang
package verify

const (
  codeAttributeNotExists sferr.ErrCode = 1000
  codeDuplicatedColumns                = 1001
)

var (
  ErrAttributeNotExists = sferr.New(codeAttributeNotExists, "The attribute %s does not exists.")
  ErrDuplicatedColumns = sferr.New(codeDuplicatedColumns, "Found duplicated columns: %s.")
)

func Verify() error {
  // do some thing
  return ErrAttributeNotExists.Format("no_exists_attr")
}
```

### All Possible Error Types

The following table lists all possible `ErrType` and keeps updating in the feature.

| ErrType | Description|
| -- | -- |
| TypeVerify | Verifing fault on checking attributes or data schema|
| TypeSQLFS | SQLFlow File system operation errors|
| TypeCodeGen | Generating submitter program errors|
| TypeSubmitter | Submitter program runtime errors|
| TypeParser | Parsing SQL program errors|
