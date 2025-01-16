package flow

import (
	"context"
	"sync"

	"github.com/worldline-go/chore/pkg/email"
	"github.com/worldline-go/chore/pkg/registry"

	"gorm.io/gorm"
)

// NodeTypes hold new function of releated node.
var NodeTypes = make(map[string]func(context.Context, *NodesReg, NodeData, string) (Noder, error))

type NodeData struct {
	Data    map[string]interface{} `json:"data"`
	Inputs  NodeConnection         `json:"inputs"`
	Outputs NodeConnection         `json:"outputs"`
	Name    string
}

type NodeConnection = map[string]Connections

type Connections = struct {
	Connections []Connection `json:"connections"`
}

type Connection = struct {
	Node   string `json:"node"`
	Output string `json:"output"`
	// input from which output information not needed
}

type Starts = struct {
	Connection
	Tags map[string]struct{}
}

// NodesData is content's represent.
type NodesData = map[string]NodeData

type Inputs struct {
	Node      string
	InputName string
	Active    bool
}

// NodeRet simple return.
type NodeRet interface {
	GetBinaryData() []byte
}

type NodeRetValues interface {
	GetBinaryValues() []byte
}

// NodeRetDatas usuful for-loop operation.
type NodeRetDatas interface {
	GetBinaryDatas() [][]byte
}

// NodeRetRespond using for responding request.
type NodeRetRespond interface {
	GetRespond() Respond
}

// NodeRetRespondData using if next node wants to respond data.
type NodeRetRespondData interface {
	GetRespondData() Respond
}

// NodeRetSelection usable if more than one output and want to choice between of them.
// Write to output numbers 0-4-5.
type NodeRetSelection interface {
	GetSelection() []int
}

// NodeDirectGo for direct go.
type NodeDirectGo interface {
	IsDirectGo() NodeRet
}

type NodeAttachments interface {
	GetAttachments() []email.Attach
}

// Noder for nodes like script, endpoint.
type Noder interface {
	GetType() string
	Run(context.Context, *sync.WaitGroup, *registry.Registry, NodeRet, string) (NodeRet, error)
	Fetch(context.Context, *gorm.DB) error
	IsFetched() bool
	Validate(context.Context) error
	ActiveInput(string, map[string]struct{})
	Next(int) []Connection
	NextCount() int
	IsRespond() bool
	Check()
	IsChecked() bool
	IsDisabled() bool
	NodeID() string
	Tags() []string
}

type NoderEndpoint interface {
	Endpoint() string
	Methods() []string
	Tags() []string
}

// nodeRetOutput struct for path.
type nodeRetOutput struct {
	output []byte
}

func (r *nodeRetOutput) GetBinaryData() []byte {
	return r.output
}
