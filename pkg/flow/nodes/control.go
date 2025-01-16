package nodes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/registry"

	"gorm.io/gorm"
)

var controlType = "control"

type ControlRet struct {
	respond flow.Respond
}

func (r *ControlRet) GetRespondData() flow.Respond {
	return r.respond
}

func (r *ControlRet) GetBinaryData() []byte {
	return r.respond.Data
}

var (
	_ flow.NodeRetRespondData = (*ControlRet)(nil)
	_ flow.NodeRet            = (*ControlRet)(nil)
)

// Control node has one input and one output.
type Control struct {
	controlName  string
	endpointName string
	methodName   string
	inputs       []flow.Inputs
	outputs      [][]flow.Connection
	checked      bool
	disabled     bool
	nodeID       string
	tags         []string
	control      models.Control
}

// Run get values from active input nodes and it will not run until last input comes.
func (n *Control) Run(ctx context.Context, wg *sync.WaitGroup, reg *registry.Registry, value flow.NodeRet, _ string) (flow.NodeRet, error) {
	content, err := base64.StdEncoding.DecodeString(n.control.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content; %w", err)
	}

	log.Ctx(ctx).Info().Msgf("internal call control=[%s] endpoint=[%s]", n.control.Name, n.endpointName)

	nodesReg, err := flow.StartFlow(ctx, wg, n.control.Name, n.endpointName, n.methodName, content, reg, value.GetBinaryData())
	if errors.Is(err, flow.ErrEndpointNotFound) {
		return nil, fmt.Errorf("endpoint not found %s; %w", n.endpointName, err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to start [control:%s;endpoint:%s] content; %w", n.controlName, n.endpointName, err)
	}

	respondChan := nodesReg.GetChan()
	if respondChan == nil {
		return nil, nil
	}

	valueChan := <-respondChan

	return &ControlRet{valueChan}, nil
}

func (n *Control) Special(_ interface{}) interface{} {
	return nil
}

func (n *Control) GetType() string {
	return controlType
}

func (n *Control) Fetch(ctx context.Context, db *gorm.DB) error {
	query := db.WithContext(ctx).Where("name = ?", n.controlName)
	result := query.First(&n.control)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("control not found %s; %w", n.controlName, result.Error)
	}

	if result.Error != nil {
		return fmt.Errorf("failed to fetch %s; %w", n.controlName, result.Error)
	}
	return nil
}

func (n *Control) IsFetched() bool {
	return true
}

func (n *Control) IsRespond() bool {
	return false
}

func (n *Control) Validate(_ context.Context) error {
	return nil
}

func (n *Control) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Control) NextCount() int {
	return len(n.outputs)
}

func (n *Control) ActiveInput(_ string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true
	}
}

func (n *Control) IsDisabled() bool {
	return n.disabled
}

func (n *Control) Check() {
	n.checked = true
}

func (n *Control) IsChecked() bool {
	return n.checked
}

func (n *Control) NodeID() string {
	return n.nodeID
}

func (n *Control) Tags() []string {
	return n.tags
}

func NewControl(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)
	outputs := flow.PrepareOutputs(data.Outputs)

	controlName, _ := data.Data["control"].(string)
	endpointName, _ := data.Data["endpoint"].(string)
	methodName, _ := data.Data["method"].(string)

	tags := convert.GetList(data.Data["tags"])

	return &Control{
		inputs:       inputs,
		outputs:      outputs,
		controlName:  controlName,
		endpointName: endpointName,
		methodName:   methodName,
		nodeID:       nodeID,
		tags:         tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[controlType] = NewControl
}
