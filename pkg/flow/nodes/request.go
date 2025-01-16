package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/rytsh/mugo/pkg/templatex"
	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/request"
	"github.com/worldline-go/chore/pkg/transfer"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

var requestType = "request"

type inputHolderRequest struct {
	value []byte
	exist bool
}

type RequestRet struct {
	selection []int
	respond   flow.Respond
}

func (r *RequestRet) GetBinaryData() []byte {
	return r.respond.Data
}

func (r *RequestRet) GetSelection() []int {
	return r.selection
}

func (r *RequestRet) GetRespondData() flow.Respond {
	return r.respond
}

var (
	_ flow.NodeRetRespondData = &RequestRet{}
	_ flow.NodeRetSelection   = &RequestRet{}
)

type retryRaw struct {
	Codes   string
	DeCodes string
}

type renderedValues struct {
	url           string
	addHeadersRaw string
	method        string
}

// Request node has one input and one output.
type Request struct {
	reg                *flow.NodesReg
	nodeID             string
	lockCtx            context.Context
	lockCancel         context.CancelFunc
	lockFeedBack       context.Context
	lockFeedBackCancel context.CancelFunc
	feedbackWait       bool
	headers            map[string]interface{}
	oauth2             request.AuthConfig
	retryRaw           retryRaw
	url                string
	addHeadersRaw      string
	method             string
	auth               string
	outputs            [][]flow.Connection
	inputs             []flow.Inputs
	inputHolder        inputHolderRequest
	mutex              sync.Mutex
	fetched            bool
	checked            bool
	disabled           bool
	payloadNil         bool
	skipVerify         bool
	retryDisabled      bool
	oauth2Name         string
	proxy              string
	stuckContext       context.Context
	log                *zerolog.Logger
	client             *request.Client
	tags               []string
}

// Run get values from active input nodes and it will not run until last input comes.
func (n *Request) Run(ctx context.Context, _ *sync.WaitGroup, reg *registry.Registry, value flow.NodeRet, input string) (flow.NodeRet, error) {
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
				return nil, fmt.Errorf("stuck detected, terminated node request")
			case <-ctx.Done():
				log.Ctx(ctx).Warn().Msg("program closed, terminated node request")

				return nil, flow.ErrStopGoroutine
			case <-n.lockCtx.Done():
				// continue process
			}
		}
	}

	// check value and render it
	rendered := renderedValues{
		url:           n.url,
		addHeadersRaw: n.addHeadersRaw,
		method:        n.method,
	}

	var requestValues interface{}
	if useValues != nil {
		requestValues = transfer.BytesToData(useValues)
	} else {
		requestValues = transfer.BytesToData(n.inputHolder.value)
	}

	// if requestValues != nil {
	// render url
	var buf bytes.Buffer
	if err := reg.Template.Execute(templatex.WithIO(&buf), templatex.WithData(requestValues), templatex.WithContent(n.url)); err != nil {
		return nil, fmt.Errorf("template url cannot render: %w", err)
	}

	rendered.url = buf.String()

	// render method
	buf = bytes.Buffer{}
	if err := reg.Template.Execute(templatex.WithIO(&buf), templatex.WithData(requestValues), templatex.WithContent(n.method)); err != nil {
		return nil, fmt.Errorf("template method cannot render: %w", err)
	}

	rendered.method = buf.String()

	// render headers
	buf = bytes.Buffer{}
	if err := reg.Template.Execute(templatex.WithIO(&buf), templatex.WithData(requestValues), templatex.WithContent(n.addHeadersRaw)); err != nil {
		return nil, fmt.Errorf("template additional headers cannot render: %w", err)
	}

	rendered.addHeadersRaw = buf.String()

	var addHeaders map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered.addHeadersRaw), &addHeaders); err != nil {
		return nil, fmt.Errorf("faild unmarshal headers in request: %w", err)
	}

	headers := make(map[string]interface{}, len(n.headers)+len(addHeaders)+1)
	if v, _ := ctx.Value("request_id").(string); v != "" {
		headers["X-Request-Id"] = v
	}
	for k := range n.headers {
		headers[k] = n.headers[k]
	}

	for k := range addHeaders {
		headers[k] = addHeaders[k]
	}

	var payload []byte
	if !n.payloadNil {
		payload = value.GetBinaryData()
	}

	if n.client == nil {
		return nil, fmt.Errorf("http client not set")
	}

	response, err := n.client.Call(
		ctx,
		rendered.url,
		rendered.method,
		headers,
		payload,
	)
	if err != nil {
		// return nil, fmt.Errorf("failed to send request: %w", err)
		return &RequestRet{
			respond: flow.Respond{
				Header: nil,
				Data:   []byte(fmt.Sprint(err)),
				Status: http.StatusServiceUnavailable,
			},
			selection: []int{0, 2},
		}, nil
	}

	header := make(map[string]interface{})
	for k, v := range response.Header {
		header[k] = v[0]
	}

	fileName := ""
	if v, _ := header["Content-Disposition"].(string); v != "" {
		_, params, err := mime.ParseMediaType(v)
		if err == nil {
			fileName = params["filename"]
		}
	}

	if response.StatusCode >= 100 && response.StatusCode < 400 {
		return &RequestRet{
			respond: flow.Respond{
				Header:   header,
				FileName: fileName,
				Data:     response.Body,
				Status:   response.StatusCode,
			},
			selection: []int{1, 2},
		}, nil
	}

	return &RequestRet{
		respond: flow.Respond{
			Header:   header,
			FileName: fileName,
			Data:     response.Body,
			Status:   response.StatusCode,
		},
		selection: []int{0, 2},
	}, nil
}

func (n *Request) GetType() string {
	return requestType
}

func (n *Request) Fetch(ctx context.Context, db *gorm.DB) error {
	if n.auth != "" {
		getData := models.AuthPure{}

		query := db.WithContext(ctx).Model(&models.Auth{}).Where("name = ?", n.auth)
		result := query.First(&getData)

		if result.Error != nil {
			return fmt.Errorf("request fetch failed: %w", result.Error)
		}

		n.headers = getData.Headers
	}

	// get oauth2 specs
	if n.oauth2Name != "" {
		authConfig := request.AuthConfig{
			Enabled: true,
		}

		data := map[string]interface{}{}
		query := db.WithContext(ctx).Model(&models.Settings{}).Where("namespace = ?", "oauth2").Where("name = ?", n.oauth2Name)
		result := query.First(&data)
		if result.Error != nil {
			return fmt.Errorf("request fetch failed: %w", result.Error)
		}

		dataInner, _ := data["data"].(string)

		if err := json.Unmarshal([]byte(dataInner), &authConfig); err != nil {
			return fmt.Errorf("request fetch failed: %w", err)
		}

		n.oauth2 = authConfig
	}

	// fill retry values
	retryCodes, err := getCodes(n.retryRaw.Codes)
	if err != nil {
		return err
	}

	retryDeCodes, err := getCodes(n.retryRaw.DeCodes)
	if err != nil {
		return err
	}

	n.client, err = request.NewClient(request.Config{ //nolint:contextcheck // application context using
		SkipVerify: n.skipVerify,
		Log:        n.log,
		Retry: request.Retry{
			Enabled:             !n.retryDisabled,
			EnabledStatusCodes:  retryCodes,
			DisabledStatusCodes: retryDeCodes,
		},
		Auth:  n.oauth2,
		Proxy: n.proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	n.fetched = true

	return nil
}

func (n *Request) IsFetched() bool {
	return n.fetched
}

func (n *Request) IsRespond() bool {
	return false
}

func (n *Request) Validate(ctx context.Context) error {
	// correction
	if n.method == "" {
		n.method = "POST"
	}

	if n.url == "" {
		return fmt.Errorf("url is empty")
	}

	n.stuckContext = n.reg.GetStuctCancel(ctx)

	return nil
}

func (n *Request) Next(i int) []flow.Connection {
	return n.outputs[i]
}

func (n *Request) NextCount() int {
	return len(n.outputs)
}

func (n *Request) IsDisabled() bool {
	return n.disabled
}

func (n *Request) ActiveInput(node string, tags map[string]struct{}) {
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

func (n *Request) Check() {
	n.checked = true
}

func (n *Request) IsChecked() bool {
	return n.checked
}

func (n *Request) NodeID() string {
	return n.nodeID
}

func (n *Request) Tags() []string {
	return n.tags
}

func NewRequest(ctx context.Context, reg *flow.NodesReg, data flow.NodeData, nodeID string) (flow.Noder, error) {
	inputs := flow.PrepareInputs(data.Inputs)

	// add outputs with order
	outputs := flow.PrepareOutputs(data.Outputs)

	auth, _ := data.Data["auth"].(string)
	method, _ := data.Data["method"].(string)
	url, _ := data.Data["url"].(string)
	addHeadersRaw, _ := data.Data["headers"].(string)

	retryCodes, _ := data.Data["retry_codes"].(string)
	retryDeCodes, _ := data.Data["retry_decodes"].(string)

	skipVerify := convert.GetBoolean(data.Data["skip_verify"])
	payloadNil := convert.GetBoolean(data.Data["payload_nil"])
	retryDisabled := convert.GetBoolean(data.Data["retry_disabled"])

	oauth2Name, _ := data.Data["oauth2"].(string)
	proxy, _ := data.Data["proxy"].(string)

	tags := convert.GetList(data.Data["tags"])

	l := log.Ctx(ctx).With().Str("component", requestType).Logger()

	return &Request{
		reg:           reg,
		inputs:        inputs,
		outputs:       outputs,
		auth:          auth,
		method:        method,
		url:           url,
		addHeadersRaw: addHeadersRaw,
		retryRaw: retryRaw{
			Codes:   strings.ReplaceAll(retryCodes, ",", " "),
			DeCodes: strings.ReplaceAll(retryDeCodes, ",", " "),
		},
		skipVerify:    skipVerify,
		payloadNil:    payloadNil,
		retryDisabled: retryDisabled,
		log:           &l,
		nodeID:        nodeID,
		tags:          tags,
		oauth2Name:    oauth2Name,
		proxy:         proxy,
	}, nil
}

//nolint:gochecknoinits // moduler nodes
func init() {
	flow.NodeTypes[requestType] = NewRequest
}

func getCodes(codes string) ([]int, error) {
	if codes == "" {
		return nil, nil
	}

	var retryCodes []int

	for _, code := range strings.Fields(codes) {
		codeInt, err := strconv.Atoi(code)
		if err != nil {
			return nil, fmt.Errorf("value %s cannot convert to integer", code)
		}

		retryCodes = append(retryCodes, codeInt)
	}

	return retryCodes, nil
}
