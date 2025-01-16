package nodes

import (
	"context"
	"fmt"
	"sync"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var waitType = "wait"

type inputHolderWait struct {
	// value flow.NodeRet
	exist bool
}

type WaitRet struct {
	flow.NodeRet
}

func (w *WaitRet) IsDirectGo() flow.NodeRet {
	return w.NodeRet
}

var _ flow.NodeDirectGo = (*WaitRet)(nil)

// Request node has one input and one output.
type Wait struct {
	reg                *flow.NodesReg
	nodeID             string
	lockCtx            context.Context
	lockCancel         context.CancelFunc
	lockFeedBack       context.Context
	lockFeedBackCancel context.CancelFunc
	feedbackWait       bool
	outputs            [][]flow.Connection
	inputs             []flow.Inputs
	inputHolder        inputHolderWait
	mutex              sync.Mutex
	stuckContext       context.Context
	checked            bool
	disabled           bool
	tags               []string
}

// Run get values from active input nodes and it will not run until last input comes.
func (n *Wait) Run(ctx context.Context, _ *sync.WaitGroup, _ *registry.Registry, value flow.NodeRet, input string) (flow.NodeRet, error) {
	// input_2 is value for pause
	if input == flow.Input2 {
		// don't allow multiple inputs
		n.mutex.Lock()
		defer n.mutex.Unlock()

		if n.inputHolder.exist {
			return nil, flow.ErrStopGoroutine
		}

		// n.inputHolder.value = value
		n.inputHolder.exist = true

		// close context to allow to others continue process
		if n.lockCancel != nil {
			n.lockCancel()
		}

		if n.feedbackWait {
			<-n.lockFeedBack.Done()
		}

		return nil, flow.ErrStopGoroutine
	}

	if n.lockCtx == nil {
		return nil, fmt.Errorf("wait node doesn't have signal to continue")
	}

	n.feedbackWait = true
	defer n.lockFeedBackCancel()

	select {
	case <-n.lockCtx.Done():
		// continue process
	default:
		// increase count
		n.reg.UpdateStuck(flow.CountStuckIncrease, false)
		defer n.reg.UpdateStuck(flow.CountStuckDecrease, false)

		// these events not happen at same time mostly
		select {
		case <-n.stuckContext.Done():
			// wait node is special, it doesn't need to be return error
			log.Ctx(ctx).Warn().Msg("stuck detected, terminated node wait")

			return nil, flow.ErrStopGoroutine
		case <-ctx.Done():
			log.Ctx(ctx).Warn().Msg("program closed, terminated node wait")

			return nil, flow.ErrStopGoroutine
		case <-n.lockCtx.Done():
			// continue process
		}
	}

	n.reg.UpdateStuck(flow.CountStuckDecrease, true)

	return &WaitRet{value}, nil
}

func (n *Wait) GetType() string {
	return waitType
}

func (n *Wait) Fetch(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (n *Wait) IsFetched() bool {
	return true
}

func (n *Wait) IsRespond() bool {
	return false
}

func (n *Wait) Validate(ctx context.Context) error {
	n.stuckContext = n.reg.GetStuctCancel(ctx)

	return nil
}

func (n *Wait) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Wait) NextCount() int {
	return len(n.outputs)
}

func (n *Wait) IsDisabled() bool {
	return n.disabled
}

func (n *Wait) ActiveInput(node string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true

		return
	}

	for i := range n.inputs {
		if n.inputs[i].Node == node {
			if !n.inputs[i].Active {
				n.inputs[i].Active = true
				// input_1 for dynamic variable
				if n.inputs[i].InputName == flow.Input2 {
					n.lockCtx, n.lockCancel = context.WithCancel(context.Background())
					n.reg.AddCleanup(n.lockCancel)
					n.lockFeedBack, n.lockFeedBackCancel = context.WithCancel(context.Background())
					n.reg.AddCleanup(n.lockFeedBackCancel)
				}
			}
		}
	}
}

func (n *Wait) Check() {
	n.checked = true
}

func (n *Wait) IsChecked() bool {
	return n.checked
}

func (n *Wait) NodeID() string {
	return n.nodeID
}

func (n *Wait) Tags() []string {
	return n.tags
}

func NewWait(ctx context.Context, reg *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)

	// add outputs with order
	outputs := flow.PrepareOutputs(data.Outputs)

	tags := convert.GetList(data.Data["tags"])

	return &Wait{
		reg:     reg,
		inputs:  inputs,
		outputs: outputs,
		nodeID:  nodeID,
		tags:    tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[waitType] = NewWait
}
