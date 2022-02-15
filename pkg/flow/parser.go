package flow

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.test.igdcs.com/finops/nextgen/apps/tools/chore/pkg/registry"
)

func ParseData(content []byte) (NodesData, error) {
	var datas NodesData
	if err := json.Unmarshal(content, &datas); err != nil {
		return nil, fmt.Errorf("parsedata cannot unmarshal: %w", err)
	}

	return datas, nil
}

func DataToNode(ctx context.Context, controlName, startName string, datas NodesData, appStore *registry.AppStore) (*NodesReg, error) {
	reg := NewNodesReg(ctx, controlName, startName, appStore)

	for nodeNumber := range datas {
		createFunc := NodeTypes[datas[nodeNumber].Name]
		if createFunc == nil {
			return nil, fmt.Errorf("node %s not found", datas[nodeNumber].Name)
		}

		node := createFunc(datas[nodeNumber])
		reg.Set(nodeNumber, node)
	}

	return reg, nil
}