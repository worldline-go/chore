package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/rytsh/mugo/pkg/templatex"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/pkg/email"
	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/transfer"
)

var emailType = "email"

type inputHolderEmail struct {
	value []byte
	exist bool
}

type EmailRet struct {
	output []byte
}

func (r *EmailRet) GetBinaryData() []byte {
	return r.output
}

// Email node has one input.
type Email struct {
	reg                *flow.NodesReg
	stuckContext       context.Context
	lockCtx            context.Context
	lockCancel         context.CancelFunc
	lockFeedBack       context.Context
	lockFeedBackCancel context.CancelFunc
	feedbackWait       bool
	values             map[string]string
	client             email.Client
	inputs             []flow.Inputs
	inputHolder        inputHolderEmail
	mutex              sync.Mutex
	fetched            bool
	checked            bool
	disabled           bool
	nodeID             string
	tags               []string
}

// Run get values from active input nodes.
func (n *Email) Run(ctx context.Context, _ *sync.WaitGroup, reg *registry.Registry, value flow.NodeRet, input string) (flow.NodeRet, error) {
	// input_1 is value
	if input == flow.Input1 {
		// don't allow multiple inputs
		n.mutex.Lock()
		defer n.mutex.Unlock()

		if n.inputHolder.exist {
			return nil, flow.ErrStopGoroutine
		}

		n.inputHolder.value = value.GetBinaryData()
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

	// check it has value
	var useValues []byte
	if vRet, ok := value.(flow.NodeRetValues); ok {
		useValues = vRet.GetBinaryValues()
	}

	if useValues == nil && n.lockCtx != nil {
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
				return nil, fmt.Errorf("stuck detected, terminated node email")
			case <-ctx.Done():
				log.Ctx(ctx).Warn().Msg("program closed, terminated node email")

				return nil, flow.ErrStopGoroutine
			case <-n.lockCtx.Done():
				// continue process
			}
		}

		n.reg.UpdateStuck(flow.CountStuckDecrease, true)
	}

	// check value and render it
	headers := make(map[string][]string)

	var requestValues interface{}
	if useValues != nil {
		requestValues = transfer.BytesToData(useValues)
	} else {
		requestValues = transfer.BytesToData(n.inputHolder.value)
	}

	for key, value := range n.values {
		payload := value

		if requestValues != nil {
			// render
			var buf bytes.Buffer
			err := reg.Template.Execute(templatex.WithIO(&buf), templatex.WithData(requestValues), templatex.WithContent(value))
			if err != nil {
				return nil, fmt.Errorf("template cannot render: %w", err)
			}

			payload = buf.String()
		}

		if key == "Subject" {
			headers[key] = []string{payload}

			continue
		}

		payloadSlice := strings.Fields(strings.ReplaceAll(payload, ",", " "))
		if len(payloadSlice) > 0 {
			headers[key] = payloadSlice
		}
	}

	var attachments []email.Attach
	if v, ok := value.(flow.NodeAttachments); ok {
		attachments = v.GetAttachments()
	}

	if err := n.client.Send(value.GetBinaryData(), headers, attachments); err != nil {
		return nil, fmt.Errorf("failed to send email: values %v, err %w", headers, err)
	}

	return &EmailRet{output: value.GetBinaryData()}, nil
}

func (n *Email) GetType() string {
	return emailType
}

func (n *Email) Fetch(ctx context.Context, db *gorm.DB) error {
	getData := map[string]interface{}{}

	query := db.WithContext(ctx).Model(&models.Settings{}).Select("data").Where("namespace = ?", "email").Where("name = ?", "email-1")
	result := query.First(&getData)

	if result.Error != nil {
		return fmt.Errorf("email fetch failed: %w", result.Error)
	}

	// log.Debug().Msgf("%v", getData)

	dataInner, _ := getData["data"].(string)
	emailModel := models.Email{}
	if err := json.Unmarshal([]byte(dataInner), &emailModel); err != nil {
		return fmt.Errorf("request fetch failed: %w", err)
	}

	port, err := strconv.ParseInt(emailModel.Port, 10, 32)
	if err != nil {
		return fmt.Errorf("request fetch failed: %w", err)
	}

	n.client = email.NewClient(emailModel.Host, int(port), emailModel.NoAuth, emailModel.Email, emailModel.Password)

	if n.values["From"] == "" {
		n.values["From"] = emailModel.Email
	}

	n.fetched = true

	return nil
}

func (n *Email) IsFetched() bool {
	return n.fetched
}

func (n *Email) IsRespond() bool {
	return false
}

func (n *Email) Validate(ctx context.Context) error {
	n.stuckContext = n.reg.GetStuctCancel(ctx)

	return nil
}

func (n *Email) Next(int) []flow.Connection {
	return nil
}

func (n *Email) NextCount() int {
	return 0
}

func (n *Email) Check() {
	n.checked = true
}

func (n *Email) IsChecked() bool {
	return n.checked
}

func (n *Email) IsDisabled() bool {
	return n.disabled
}

func (n *Email) ActiveInput(node string, tags map[string]struct{}) {
	if !convert.IsTagsEnabled(n.tags, tags) {
		n.disabled = true
		return
	}

	for i := range n.inputs {
		if n.inputs[i].Node == node {
			if !n.inputs[i].Active {
				n.inputs[i].Active = true
				// input_1 for dynamic variable
				if n.inputs[i].InputName == flow.Input1 {
					n.lockCtx, n.lockCancel = context.WithCancel(context.Background())
					n.reg.AddCleanup(n.lockCancel)
					n.lockFeedBack, n.lockFeedBackCancel = context.WithCancel(context.Background())
					n.reg.AddCleanup(n.lockFeedBackCancel)
				}
			}
		}
	}
}

func (n *Email) NodeID() string {
	return n.nodeID
}

func (n *Email) Tags() []string {
	return n.tags
}

func NewEmail(_ context.Context, reg *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)

	values := make(map[string]string)

	values["From"], _ = data.Data["from"].(string)
	values["To"], _ = data.Data["to"].(string)
	values["Cc"], _ = data.Data["cc"].(string)
	values["Bcc"], _ = data.Data["bcc"].(string)
	values["Subject"], _ = data.Data["subject"].(string)

	tags := convert.GetList(data.Data["tags"])

	return &Email{
		reg:    reg,
		values: values,
		inputs: inputs,
		nodeID: nodeID,
		tags:   tags,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[emailType] = NewEmail
}
