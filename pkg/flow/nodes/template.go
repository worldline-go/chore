package nodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/rytsh/mugo/pkg/templatex"
	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/transfer"

	"gorm.io/gorm"
)

var templateType = "template"

type TemplateRet struct {
	output []byte
}

func (r *TemplateRet) GetBinaryData() []byte {
	return r.output
}

// Template node has one input and one output.
type Template struct {
	templateName string
	inputs       []flow.Inputs
	outputs      [][]flow.Connection
	content      []byte
	fetched      bool
	checked      bool
	disabled     bool
	nodeID       string
	tags         []string
}

// Run get values from active input nodes and it will not run until last input comes.
func (n *Template) Run(_ context.Context, _ *sync.WaitGroup, reg *registry.Registry, value flow.NodeRet, _ string) (flow.NodeRet, error) {
	v := transfer.BytesToData(value.GetBinaryData())

	buf := bytes.Buffer{}
	if err := reg.Template.Execute(templatex.WithIO(&buf), templatex.WithData(v), templatex.WithContent(string(n.content))); err != nil {
		return nil, fmt.Errorf("template cannot render: %w", err)
	}

	return &TemplateRet{buf.Bytes()}, nil
}

func (n *Template) Special(_ interface{}) interface{} {
	return nil
}

func (n *Template) GetType() string {
	return templateType
}

func (n *Template) Fetch(ctx context.Context, db *gorm.DB) error {
	if n.templateName == "" {
		return fmt.Errorf("template fetch failed: templateName empty")
	}

	getData := models.TemplatePure{}

	query := db.WithContext(ctx).Model(&models.Template{}).Where("name = ?", n.templateName)
	result := query.First(&getData)

	if result.Error != nil {
		return fmt.Errorf("template fetch failed: %w", result.Error)
	}

	content, err := base64.StdEncoding.DecodeString(getData.Content)
	if err != nil {
		return fmt.Errorf("template fetch failed: %w", err)
	}

	n.content = content

	n.fetched = true

	return nil
}

func (n *Template) IsFetched() bool {
	return n.fetched
}

func (n *Template) IsRespond() bool {
	return false
}

func (n *Template) Validate(_ context.Context) error {
	return nil
}

func (n *Template) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Template) NextCount() int {
	return len(n.outputs)
}

func (n *Template) IsDisabled() bool {
	return n.disabled
}

func (n *Template) ActiveInput(_ string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true

		return
	}
}

func (n *Template) Check() {
	n.checked = true
}

func (n *Template) IsChecked() bool {
	return n.checked
}

func (n *Template) NodeID() string {
	return n.nodeID
}

func (n *Template) Tags() []string {
	return n.tags
}

func NewTemplate(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)

	// add outputs with order
	outputs := flow.PrepareOutputs(data.Outputs)

	templateName, _ := data.Data["template"].(string)
	tags := convert.GetList(data.Data["tags"])

	return &Template{
		inputs:       inputs,
		outputs:      outputs,
		templateName: templateName,
		nodeID:       nodeID,
		tags:         tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[templateType] = NewTemplate
}
