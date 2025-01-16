package nodes

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/script/js"
	"github.com/worldline-go/chore/pkg/transfer"
)

var ifCaseType = "ifCase"

type IfRet struct {
	output    []byte
	selection []int
}

func (r *IfRet) GetBinaryData() []byte {
	return r.output
}

func (r *IfRet) GetSelection() []int {
	return r.selection
}

var _ flow.NodeRetSelection = (*IfRet)(nil)

// Ifcase node has one input and one output.
// Not need to wait other inputs.
type IfCase struct {
	expression string
	outputs    [][]flow.Connection
	checked    bool
	disabled   bool
	nodeID     string
	tags       []string
}

// selection 0 is false.
func (n *IfCase) Run(ctx context.Context, _ *sync.WaitGroup, _ *registry.Registry, value flow.NodeRet, input string) (flow.NodeRet, error) {
	var transferValue interface{}
	if value.GetBinaryData() != nil {
		transferValue = transfer.BytesToData(value.GetBinaryData())
	}

	runner := js.NewGoja()

	if err := runner.SetData(transferValue); err != nil {
		return nil, fmt.Errorf("cannot set data in script: %w", err)
	}

	gojaV, err := runner.RunString(n.expression)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("cannot run loop value, passing as false: %v", err)

		return &IfRet{
			output:    value.GetBinaryData(),
			selection: []int{0},
		}, nil
	}

	if gojaV.ToBoolean() {
		return &IfRet{
			output:    value.GetBinaryData(),
			selection: []int{1},
		}, nil
	}

	return &IfRet{
		output:    value.GetBinaryData(),
		selection: []int{0},
	}, nil
}

func (n *IfCase) GetType() string {
	return ifCaseType
}

func (n *IfCase) Fetch(_ context.Context, _ *gorm.DB) error {
	return nil
}

func (n *IfCase) IsFetched() bool {
	return true
}

func (n *IfCase) IsRespond() bool {
	return false
}

func (n *IfCase) Validate(_ context.Context) error {
	return nil
}

func (n *IfCase) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *IfCase) NextCount() int {
	return len(n.outputs)
}

func (n *IfCase) IsDisabled() bool {
	return n.disabled
}

func (n *IfCase) ActiveInput(_ string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true

		return
	}
}

func (n *IfCase) Check() {
	n.checked = true
}

func (n *IfCase) IsChecked() bool {
	return n.checked
}

func (n *IfCase) NodeID() string {
	return n.nodeID
}

func (n *IfCase) Tags() []string {
	return n.tags
}

func NewIfCase(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	// add outputs with order
	outputs := flow.PrepareOutputs(data.Outputs)

	expression, _ := data.Data["if"].(string)
	tags := convert.GetList(data.Data["tags"])

	return &IfCase{
		outputs:    outputs,
		expression: expression,
		nodeID:     nodeID,
		tags:       tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[ifCaseType] = NewIfCase
}
