package flow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/worldline-go/chore/pkg/registry"
)

func ParseData(content []byte) (NodesData, error) {
	var datas NodesData
	if err := json.Unmarshal(content, &datas); err != nil {
		return nil, fmt.Errorf("parsedata cannot unmarshal: %w", err)
	}

	return datas, nil
}

func DataToNode(
	ctx context.Context,
	controlName, startName, method string,
	datas NodesData,
	appStore *registry.Registry,
) (*NodesReg, error) {
	reg := NewNodesReg(controlName, startName, method, appStore)

	for nodeNumber := range datas {
		createFunc := NodeTypes[datas[nodeNumber].Name]
		if createFunc == nil {
			continue
		}

		node, err := createFunc(ctx, reg, datas[nodeNumber], nodeNumber)
		if err != nil {
			return nil, err
		}

		reg.Set(nodeNumber, node)
	}

	return reg, nil
}
