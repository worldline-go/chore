package nodes

import (
	"context"
	"sync"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"

	"gorm.io/gorm"
)

var hubType = "hub"

type HubRet struct {
	flow.NodeRet
}

func (w *HubRet) IsDirectGo() flow.NodeRet {
	return w.NodeRet
}

// Respond node has one input.
type Hub struct {
	outputs  [][]flow.Connection
	checked  bool
	disabled bool
	nodeID   string
	tags     []string
}

// Run get values from everywhere no need to check active input.
func (n *Hub) Run(ctx context.Context, _ *sync.WaitGroup, _ *registry.Registry, value flow.NodeRet, _ string) (flow.NodeRet, error) {
	return &HubRet{NodeRet: value}, nil
}

func (n *Hub) Special(_ interface{}) interface{} {
	return nil
}

func (n *Hub) GetType() string {
	return hubType
}

func (n *Hub) Fetch(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (n *Hub) IsFetched() bool {
	return true
}

func (n *Hub) IsRespond() bool {
	return false
}

func (n *Hub) Validate(_ context.Context) error {
	return nil
}

func (n *Hub) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Hub) NextCount() int {
	return len(n.outputs)
}

func (n *Hub) Check() {
	n.checked = true
}

func (n *Hub) IsChecked() bool {
	return n.checked
}

func (n *Hub) IsDisabled() bool {
	return n.disabled
}

func (n *Hub) ActiveInput(_ string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true

		return
	}
}

func (n *Hub) NodeID() string {
	return n.nodeID
}

func (n *Hub) Tags() []string {
	return n.tags
}

func NewHub(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	outputs := flow.PrepareOutputs(data.Outputs)

	tags := convert.GetList(data.Data["tags"])

	return &Hub{
		outputs: outputs,
		nodeID:  nodeID,
		tags:    tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[hubType] = NewHub
}
