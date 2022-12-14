package flow

import (
	"context"
	"strings"
	"sync"

	"github.com/worldline-go/chore/pkg/registry"
)

type Respond struct {
	Header  map[string]interface{} `json:"header"`
	Data    []byte                 `json:"data"`
	Status  int                    `json:"status"`
	IsError bool                   `json:"-"`
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
	appStore          *registry.AppStore
	reg               map[string]Noder
	respondChan       chan Respond
	controlName       string
	startName         string
	method            string
	starts            []Connection
	mutex             sync.RWMutex
	wgx               sync.WaitGroup
	respondChanActive bool
	errors            []error
	mutexErr          sync.RWMutex
	// prevent stuck operation
	totalCount int64
	stuckCount int64
	mutexCount sync.Mutex
	stuckCtx   context.Context
	stuckChan  chan bool
}

func NewNodesReg(ctx context.Context, controlName, startName, method string, appStore *registry.AppStore) *NodesReg {
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

// GetStuckChan return nil if not started.
func (r *NodesReg) GetStuckCtx() context.Context {
	return r.stuckCtx
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

	// log.Debug().Msgf("total: %d, stuck: %d", r.totalCount, r.stuckCount)

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
	r.mutexErr.Lock()
	defer r.mutexErr.Unlock()

	r.errors = append(r.errors, err)
}

func (r *NodesReg) Get(number string) (Noder, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	node, ok := r.reg[number]

	return node, ok
}

func (r *NodesReg) GetStarts() []Connection {
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
					r.starts = append(r.starts, Connection{
						Node: number,
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
