# SQLFlow Error Diagnosis

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

To improve the error message maintainability and readability, this document issues an SQLFlow Error package named `sferr` to maintain various error messages in a rigorous approach.

## Error Package

We would like each Go package under `pkg` folder do one minimal thing, for each package, we represent different `ErrType`, and using `ErrCode` to
distinguish different errors for the same `ErrType`, the error code can be the same among all `ErrType`s.

``` golang
// ErrType represents a specific Error Type
type ErrType int

// ErrCode represents a specific Error Type in a ErrType
type ErrCode int

type Error struct {
  typ     ErrType
  code    ErrCode
  format  string
}

func (e Error) FormatWithArg(a ...interface{}) error {
  return fmt.Errorf("[%s:%d] %s", typ, code, fmt.Sprintf(format, a))
}
```

In order to use error management more rigorously, we maintain a `ErrType` list in `sferr` package as the following:

``` golang
const (
  TypeVerify ErrType iota
  TypeSQLFS
)

func (t ErrType) New(code ErrCode, format string) Error {
  return &Error{
    typ:      t,
    code:     code,
    format:   format,
  }
}

```

### An Example of Usage

We want uses to use `sferr` package as the following code:

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

func attributeChecker() error {
  // do some thing
  return ErrAttributeNotExists.FormatWithArg("no_exists_attribute")
}
```
