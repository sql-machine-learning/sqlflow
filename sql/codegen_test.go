package sql

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT MonthlyCharges, TotalCharges, tenure
FROM churn.churn
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 73,
  hidden_units = [10, 20]
COLUMN MonthlyCharges, TotalCharges
LABEL tenure
INTO
  my_dnn_model
;
`
	simpleInferSelect = simpleSelect + `INFER my_dnn_model;`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	tpl, ok := NewTemplateFiller(&parseResult, fts, testCfg)
	a.Equal(true, ok)

	var text bytes.Buffer
	err := codegen_template.Execute(&text, tpl)
	if err != nil {
		log.Println("executing template:", err)
	}
	a.Equal(err, nil)

	cmd := tensorflowCmd()
	cmd.Stdin = bytes.NewReader(text.Bytes())
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	a.True(strings.ContainsAny(string(o), "Done training"))
}

func tryRun(cmd string, args ...string) bool {
	if exec.Command(cmd, args...).Run() != nil {
		return false
	}
	return true
}

func hasPython() bool {
	return tryRun("python", "-V")
}

func hasTensorFlow() bool {
	return tryRun("python", "-c", "import tensorflow")
}

func hasDocker() bool {
	return tryRun("docker", "version")
}

func hasDockerImage(image string) bool {
	b, e := exec.Command("docker", "images", "-q", image).Output()
	if e != nil || len(b) == 0 {
		return false
	}
	return true
}

func tensorflowCmd() (cmd *exec.Cmd) {
	if hasPython() && hasTensorFlow() {
		cmd = exec.Command("python")
	} else if hasDocker() {
		const tfImg = "tensorflow/tensorflow:1.12.0"
		if !hasDockerImage(tfImg) {
			log.Printf("No local Docker image %s.  It will take a long time to pull.", tfImg)
		}
		cmd = exec.Command("docker", "run", "--rm", "--network=host", "-i", tfImg, "python")
	} else {
		log.Fatalf("No local TensorFlow or Docker.  No way to run TensorFlow programs")
	}
	return cmd
}
