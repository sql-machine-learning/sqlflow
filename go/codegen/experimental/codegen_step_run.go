package experimental

import (
	"bytes"
	"html/template"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type runStepFilter struct {
	StepIndex  int
	DataSource string
	Select     string
	ImageName  string
	Parameters string
	Into       string
	Submitter  string
}

// GenerateRun returns the step code for TO RUN.
func GenerateRun(runStmt *ir.RunStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filter := &runStepFilter{
		StepIndex:  stepIndex,
		DataSource: ds,
		Select:     runStmt.Select,
		ImageName:  runStmt.ImageName,
		Parameters: ir.AttrToPythonValue(runStmt.Parameters),
		Into:       runStmt.Into,
		Submitter:  getSubmitter(session),
	}

	var program bytes.Buffer
	var runTemplate = template.Must(template.New("Run").Parse(runStepTemplate))
	err = runTemplate.Execute(&program, filter)
	if err != nil {
		return "", err
	}

	return program.String(), nil
}

const runStepTemplate = `
def step_entry_{{.StepIndex}}():
    from runtime.{{.Submitter}} import run

    run(datasource='''{{.DataSource}}''',
        select='''{{.Select}}''',
        image_name='''{{.ImageName}}''',
        params='''{{.Parameters}}''',
        into='''{{.Into}}''')
`
