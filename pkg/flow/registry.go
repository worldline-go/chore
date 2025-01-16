package flow

import (
	"context"
	"strings"
	"sync"

	"github.com/worldline-go/chore/pkg/flow/convert"
	"github.com/worldline-go/chore/pkg/registry"
)

type Respond struct {
	Header   map[string]interface{} `json:"header"`
	FileName string                 `json:"-"`
	Data     []byte                 `json:"data"`
	Status   int                    `json:"status"`
	IsError  bool                   `json:"-"`
}

type CountStucker uint

const (
	CountTotalIncrease CountStucker = iota + 1
	CountTotalDecrease
	CountStuckIncrease
	CountStuckDecrease
)

// NodesReg hold concreate information of nodes and start points.
type NodesReg struct {
	appStore          *registry.Registry
	reg               map[string]Noder
	respondChan       chan Respond
	controlName       string
	startName         string
	method            string
	starts            []Starts
	mutex             sync.RWMutex
	wgx               sync.WaitGroup
	respondChanActive bool
	errors            []error
	// prevent stuck operation
	totalCount      int64
	stuckCount      int64
	mutexCount      sync.Mutex
	stuckCtxCancels []context.CancelFunc
	cleanup         []func()
	stuckChan       chan bool
}

func NewNodesReg(controlName, startName, method string, appStore *registry.Registry) *NodesReg {
	return &NodesReg{
		controlName: controlName,
		startName:   startName,
		method:      method,
		reg:         make(map[string]Noder),
		appStore:    appStore,
	}
}

func (r *NodesReg) GetChan() <-chan Respond {
	if r.respondChanActive {
		return r.respondChan
	}

	return nil
}

// CancelStucks cancel all stuck context of nodes and clear stuck context list.
func (r *NodesReg) CancelStucks() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, cancel := range r.stuckCtxCancels {
		cancel()
	}

	r.stuckCtxCancels = nil
}

func (r *NodesReg) AddStuctCancel(v context.CancelFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.stuckCtxCancels = append(r.stuckCtxCancels, v)
}

func (r *NodesReg) GetStuctCancel(ctx context.Context) context.Context {
	ctxStuct, ctxCancel := context.WithCancel(ctx)
	r.AddStuctCancel(ctxCancel)

	return ctxStuct
}

// Clear clear all context.
func (r *NodesReg) Clear() {
	for _, cancel := range r.stuckCtxCancels {
		cancel()
	}

	for _, v := range r.cleanup {
		v()
	}
}

func (r *NodesReg) AddCleanup(v func()) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.cleanup = append(r.cleanup, v)
}

func (r *NodesReg) UpdateStuck(typeCount CountStucker, trigger bool) {
	r.mutexCount.Lock()
	defer r.mutexCount.Unlock()

	switch typeCount {
	case CountTotalIncrease:
		r.totalCount++
	case CountTotalDecrease:
		r.totalCount--
	case CountStuckIncrease:
		r.stuckCount++
	case CountStuckDecrease:
		r.stuckCount--
	}

	// log.Info().Msgf("total: %d, stuck: %d, trigger: %v", r.totalCount, r.stuckCount, trigger)

	if trigger {
		r.stuckChan <- r.totalCount-r.stuckCount == 0
	}
}

func (r *NodesReg) SetChanInactive() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.respondChanActive = false
}

func (r *NodesReg) AddError(err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.errors = append(r.errors, err)
}

func (r *NodesReg) Get(number string) (Noder, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	node, ok := r.reg[number]

	return node, ok
}

func (r *NodesReg) GetStarts() []Starts {
	return r.starts
}

// Set a concreate node to registry.
// Number is a node number like 2, 4.
func (r *NodesReg) Set(number string, node Noder) {
	// checkdata usable for starter nodes like endpoint
	if nodeEndpoint, ok := node.(NoderEndpoint); ok {
		if nodeEndpoint.Endpoint() == r.startName {
			for _, v := range nodeEndpoint.Methods() {
				if strings.ToUpper(strings.TrimSpace(v)) == r.method {
					r.starts = append(r.starts, Starts{
						Connection: Connection{
							Node: number,
						},
						Tags: convert.SliceToMap(nodeEndpoint.Tags()),
					})

					break
				}
			}
		}
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.reg[number] = node
}
