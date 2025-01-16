package nodes

import (
	"bytes"
	"context"
	"sort"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/worldline-go/chore/pkg/email"
	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/script/js"
	"github.com/worldline-go/chore/pkg/transfer"

	"gorm.io/gorm"
)

var scriptType = "script"

type inputHolderS struct {
	value interface{}
	input string
}

type ScriptRet struct {
	selection    []int
	output       []byte
	outputValues []byte
	attachments  []email.Attach
}

func (r *ScriptRet) GetBinaryData() []byte {
	return r.output
}

func (r *ScriptRet) GetSelection() []int {
	return r.selection
}

func (r *ScriptRet) GetBinaryValues() []byte {
	return r.outputValues
}

func (r *ScriptRet) GetAttachments() []email.Attach {
	return r.attachments
}

var (
	_ flow.NodeRetSelection = (*ScriptRet)(nil)
	_ flow.NodeRetValues    = (*ScriptRet)(nil)
	_ flow.NodeAttachments  = (*ScriptRet)(nil)
)

// Script node has many input and one output.
type Script struct {
	script       string
	inputs       []flow.Inputs
	inputsAll    []string
	inputCounter map[string]struct{}
	inputHolder  map[string]inputHolderS
	inputRequest map[string]flow.Respond
	outputs      [][]flow.Connection
	lock         sync.Mutex
	checked      bool
	disabled     bool
	nodeID       string
	tags         []string
}

// selection 0 is false.
//
//nolint:lll // false positive
func (n *Script) Run(ctx context.Context, _ *sync.WaitGroup, _ *registry.Registry, value flow.NodeRet, input string) (flow.NodeRet, error) {
	var transferValue interface{}
	if v := value.GetBinaryData(); v != nil {
		transferValue = transfer.BytesToData(v)
	}

	var inputValues []inputHolderS

	if len(n.inputCounter) > 1 {
		n.lock.Lock()

		// add request data to script
		if v, _ := value.(flow.NodeRetRespondData); v != nil {
			n.inputRequest[input] = v.GetRespondData()
		}

		// replace input value, multiple call on same input will replace value
		n.inputHolder[input] = inputHolderS{value: transferValue, input: input}

		if len(n.inputCounter) > len(n.inputHolder) {
			n.lock.Unlock()

			return nil, flow.ErrStopGoroutine
		}

		inputValues = make([]inputHolderS, 0, len(n.inputHolder))
		for _, v := range n.inputHolder {
			inputValues = append(inputValues, v)
		}

		n.lock.Unlock()
	} else {
		// add request data to script
		if v, _ := value.(flow.NodeRetRespondData); v != nil {
			n.inputRequest[input] = v.GetRespondData()
		}

		n.inputHolder[input] = inputHolderS{value: transferValue, input: input}
		inputValues = []inputHolderS{n.inputHolder[input]}
	}

	// fill other inputs with nil
	for _, input := range n.inputsAll {
		if _, ok := n.inputHolder[input]; !ok {
			inputValues = append(inputValues, inputHolderS{value: nil, input: input})
		}
	}

	// sort inputholder by input name, it effects to function arguments order
	sort.Slice(inputValues, func(i, j int) bool {
		return inputValues[i].input < inputValues[j].input
	})

	// create script runner
	runner := js.NewGoja()

	// value for change template
	var valueToPass interface{}
	setValue := func(v interface{}) {
		valueToPass = v
	}

	runner.SetFunction("setValue", setValue)

	// value for email attachment
	var valueAttachment []email.Attach
	setAttachment := func(name string, v []byte) {
		valueAttachment = append(valueAttachment, email.Attach{
			FileName: name,
			Content:  bytes.NewReader(v),
		})
	}

	runner.SetFunction("setAttachment", setAttachment)

	if err := runner.Set("request", n.inputRequest); err != nil {
		log.Ctx(ctx).Warn().Msgf("cannot set data to script: %v", err)
	}

	inputValuesInterface := make([]interface{}, len(inputValues))
	for i, v := range inputValues {
		inputValuesInterface[i] = v.value
	}

	result, err := runner.RunScript(ctx, n.script, inputValuesInterface)
	if err != nil {
		return &ScriptRet{ //nolint:nilerr // different kind of error
			selection:    []int{0, 2},
			output:       result,
			outputValues: transfer.DataToBytes(valueToPass),
			attachments:  valueAttachment,
		}, nil
	}

	return &ScriptRet{
		selection:    []int{1, 2},
		output:       result,
		outputValues: transfer.DataToBytes(valueToPass),
		attachments:  valueAttachment,
	}, nil
}

func (n *Script) GetType() string {
	return scriptType
}

func (n *Script) Fetch(_ context.Context, _ *gorm.DB) error {
	return nil
}

func (n *Script) IsFetched() bool {
	return true
}

func (n *Script) IsRespond() bool {
	return false
}

func (n *Script) Validate(_ context.Context) error {
	return nil
}

func (n *Script) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Script) NextCount() int {
	return len(n.outputs)
}

func (n *Script) IsDisabled() bool {
	return n.disabled
}

func (n *Script) ActiveInput(node string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true

		return
	}

	for i := range n.inputs {
		if n.inputs[i].Node == node {
			if !n.inputs[i].Active {
				n.inputs[i].Active = true
				n.inputCounter[n.inputs[i].InputName] = struct{}{}
			}
		}
	}
}

func (n *Script) Check() {
	n.checked = true
}

func (n *Script) IsChecked() bool {
	return n.checked
}

func (n *Script) NodeID() string {
	return n.nodeID
}

func (n *Script) Tags() []string {
	return n.tags
}

func NewScript(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)
	inputsAll := flow.PrepareAllInputs(data.Inputs)

	// add outputs with order
	outputs := flow.PrepareOutputs(data.Outputs)

	script, _ := data.Data["script"].(string)
	tags := convert.GetList(data.Data["tags"])

	return &Script{
		inputs:       inputs,
		inputsAll:    inputsAll,
		outputs:      outputs,
		script:       script,
		nodeID:       nodeID,
		inputCounter: make(map[string]struct{}),
		inputHolder:  make(map[string]inputHolderS),
		inputRequest: make(map[string]flow.Respond),
		tags:         tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[scriptType] = NewScript
}
