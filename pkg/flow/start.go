package flow

import (
	"context"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/worldline-go/chore/pkg/registry"
)

func StartFlow(
	ctx context.Context,
	wg *sync.WaitGroup,
	controlName, endPoint, method string,
	content []byte,
	appStore *registry.Registry,
	value []byte,
) (*NodesReg, error) {
	nodesData, err := ParseData(content)
	if err != nil {
		return nil, err
	}

	// sanitize inputs
	controlName = strings.TrimSpace(controlName)
	endPoint = strings.TrimSpace(endPoint)
	method = strings.TrimSpace(method)

	// set new logger for reg and set it in ctx
	ctx = log.Ctx(ctx).With().Str("control", controlName).Str("endpoint", endPoint).Logger().WithContext(ctx)

	nodesReg, err := DataToNode(ctx, controlName, endPoint, method, nodesData, appStore)
	if err != nil {
		return nil, err
	}

	if err := VisitAndFetch(ctx, nodesReg); err != nil {
		return nil, err
	}

	wg.Add(1)
	go GoAndRun(ctx, wg, nodesReg, value)

	return nodesReg, nil
}
