package nodes

import (
	"context"
	"strings"
	"sync"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"

	"gorm.io/gorm"
)

var endpointType = "endpoint"

type EndpointRet struct {
	output []byte
}

func (r *EndpointRet) GetBinaryData() []byte {
	return r.output
}

// Endpoint node has one output.
type Endpoint struct {
	endpoint string
	outputs  [][]flow.Connection
	methods  []string
	checked  bool
	disabled bool
	public   bool
	nodeID   string
	tags     []string
}

var _ flow.NoderEndpoint = (*Endpoint)(nil)

// Run get values from active input nodes and it will not run until last input comes.
func (n *Endpoint) Run(_ context.Context, _ *sync.WaitGroup, _ *registry.Registry, value flow.NodeRet, _ string) (flow.NodeRet, error) {
	return &EndpointRet{output: value.GetBinaryData()}, nil
}

func (n *Endpoint) GetType() string {
	return endpointType
}

func (n *Endpoint) Fetch(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (n *Endpoint) IsFetched() bool {
	return true
}

func (n *Endpoint) IsRespond() bool {
	return false
}

func (n *Endpoint) Validate(_ context.Context) error {
	return nil
}

func (n *Endpoint) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Endpoint) NextCount() int {
	return len(n.outputs)
}

func (n *Endpoint) Check() {
	n.checked = true
}

func (n *Endpoint) IsChecked() bool {
	return n.checked
}

func (n *Endpoint) IsDisabled() bool {
	return n.disabled
}

func (n *Endpoint) ActiveInput(string, map[string]struct{}) {}

func (n *Endpoint) Endpoint() string {
	return n.endpoint
}

func (n *Endpoint) Methods() []string {
	return n.methods
}

func (n *Endpoint) Tags() []string {
	return n.tags
}

func (n *Endpoint) NodeID() string {
	return n.nodeID
}

func NewEndpoint(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	outputs := flow.PrepareOutputs(data.Outputs)

	endpoint, _ := data.Data["endpoint"].(string)
	methodsRaw, _ := data.Data["methods"].(string)
	public := convert.GetBoolean(data.Data["public"])

	methodsRaw = strings.ReplaceAll(methodsRaw, ",", " ")
	methods := strings.Fields(methodsRaw)

	tags := convert.GetList(data.Data["tags"])

	return &Endpoint{
		outputs:  outputs,
		endpoint: endpoint,
		methods:  methods,
		public:   public,
		nodeID:   nodeID,
		tags:     tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[endpointType] = NewEndpoint
}
