package nodes

import (
	"context"
	"sync"

	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/registry"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var logType = "log"

type LogRet struct {
	output []byte
}

func (r *LogRet) GetBinaryData() []byte {
	return r.output
}

// Respond node has one input.
type Log struct {
	message   string
	outputs   [][]flow.Connection
	printData bool
	logLevel  zerolog.Level
	checked   bool
	nodeID    string
}

// Run get values from everywhere no need to check active input.
func (n *Log) Run(ctx context.Context, _ *sync.WaitGroup, _ *registry.AppStore, value flow.NodeRet, _ string) (flow.NodeRet, error) {
	var logEvent *zerolog.Event

	switch n.logLevel {
	case zerolog.DebugLevel:
		logEvent = log.Ctx(ctx).Debug()
	case zerolog.InfoLevel:
		logEvent = log.Ctx(ctx).Info()
	case zerolog.WarnLevel:
		logEvent = log.Ctx(ctx).Warn()
	case zerolog.ErrorLevel:
		logEvent = log.Ctx(ctx).Error()
	default:
		logEvent = log.Ctx(ctx).WithLevel(zerolog.NoLevel)
	}

	if n.printData {
		logEvent = logEvent.Str("data", string(value.GetBinaryData()))
	}

	logEvent.Msg(n.message)

	return &LogRet{value.GetBinaryData()}, nil
}

func (n *Log) Special(_ interface{}) interface{} {
	return nil
}

func (n *Log) GetType() string {
	return logType
}

func (n *Log) Fetch(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (n *Log) IsFetched() bool {
	return true
}

func (n *Log) IsRespond() bool {
	return false
}

func (n *Log) Validate() error {
	return nil
}

func (n *Log) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Log) NextCount() int {
	return len(n.outputs)
}

func (n *Log) Check() {
	n.checked = true
}

func (n *Log) IsChecked() bool {
	return n.checked
}

func (n *Log) ActiveInput(string) {}

func (n *Log) NodeID() string {
	return n.nodeID
}

func NewLog(_ context.Context, _ *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	outputs := flow.PrepareOutputs(data.Outputs)

	// printData "true" or "false"
	printData, _ := data.Data["data"].(string)
	// loglevel "debug", "info", "warn", "error"
	level, _ := data.Data["level"].(string)
	// message default ""
	message, _ := data.Data["message"].(string)

	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.DebugLevel
	}

	return &Log{
		outputs:   outputs,
		printData: printData == "true",
		logLevel:  logLevel,
		message:   message,
		nodeID:    nodeID,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[logType] = NewLog
}
