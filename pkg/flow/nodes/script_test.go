package nodes

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-test/deep"
	"github.com/worldline-go/chore/pkg/flow"
)

func TestScript_Run(t *testing.T) {
	type args struct {
		ctx  context.Context
		data flow.NodeData
	}
	tests := []struct {
		name  string
		args  args
		datas map[string]struct {
			data  []byte
			input string
		}
		want    flow.NodeRet
		wantErr bool
	}{
		{
			name: "multiple inputs without active all",
			args: args{
				ctx: context.Background(),
				data: flow.NodeData{
					Data: map[string]interface{}{
						"script": "function main(input_1,input_2,input_3,input_4){return {output_1:input_1,output_2:input_2,output_3:input_3,output_4:input_4}}",
					},
					Inputs: flow.NodeConnection{
						"input_1": flow.Connections{
							[]flow.Connection{
								{
									Node: "1",
								},
							},
						},
						"input_2": flow.Connections{
							[]flow.Connection{
								{
									Node: "2",
								},
							},
						},
						"input_3": flow.Connections{
							[]flow.Connection{
								{
									Node: "3",
								},
							},
						},
						"input_4": flow.Connections{
							[]flow.Connection{
								{
									Node: "4",
								},
							},
						},
					},
				},
			},
			datas: map[string]struct {
				data  []byte
				input string
			}{
				"1": {
					data:  []byte("node_1"),
					input: "input_1",
				},
				// "2": {
				// 	data:  []byte("node_2"),
				// 	input: "input_2",
				// },
				"3": {
					data:  []byte("node_3"),
					input: "input_3",
				},
				"4": {
					data:  []byte("node_4"),
					input: "input_4",
				},
			},
			want: &ScriptRet{
				output: []byte(`{"output_1":"node_1","output_2":null,"output_3":"node_3","output_4":"node_4"}`),
			},
		},
		{
			name: "multiple inputs, selective active",
			args: args{
				ctx: context.Background(),
				data: flow.NodeData{
					Data: map[string]interface{}{
						"script": "function main(input_1,input_2,input_3,input_4){return {output_1:input_1,output_2:input_2,output_3:input_3,output_4:input_4}}",
					},
					Inputs: flow.NodeConnection{
						"input_1": flow.Connections{
							[]flow.Connection{
								{
									Node: "1",
								},
							},
						},
						"input_2": flow.Connections{
							[]flow.Connection{},
						},
						"input_3": flow.Connections{
							[]flow.Connection{
								{
									Node: "4",
								},
							},
						},
						"input_4": flow.Connections{
							[]flow.Connection{},
						},
					},
				},
			},
			datas: map[string]struct {
				data  []byte
				input string
			}{
				"1": {data: []byte("node_1"), input: "input_1"},
			},
			want: &ScriptRet{
				output: []byte(`{"output_1":"node_1","output_2":null,"output_3":null,"output_4":null}`),
			},
		},
		{
			name: "multiple inputs, selective active 2",
			args: args{
				ctx: context.Background(),
				data: flow.NodeData{
					Data: map[string]interface{}{
						"script": "function main(input_1,input_2,input_3,input_4){return {output_1:input_1,output_2:input_2,output_3:input_3,output_4:input_4}}",
					},
					Inputs: flow.NodeConnection{
						"input_1": flow.Connections{
							[]flow.Connection{
								{
									Node: "1",
								},
							},
						},
						"input_2": flow.Connections{
							[]flow.Connection{},
						},
						"input_3": flow.Connections{
							[]flow.Connection{
								{
									Node: "4",
								},
							},
						},
						"input_4": flow.Connections{
							[]flow.Connection{},
						},
					},
				},
			},
			// active node
			// active node data
			datas: map[string]struct {
				data  []byte
				input string
			}{
				"4": {data: []byte("node_4"), input: "input_3"},
			},
			want: &ScriptRet{
				output: []byte(`{"output_1":null,"output_2":null,"output_3":"node_4","output_4":null}`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewScript(context.TODO(), nil, tt.args.data, "test")
			if err != nil {
				t.Errorf("NewScript error = %v", err)
				return
			}

			for nodeID := range tt.datas {
				n.ActiveInput(nodeID, nil)
			}

			wg := &sync.WaitGroup{}
			for _, value := range tt.datas {
				wg.Add(1)
				got, err := n.Run(
					tt.args.ctx,
					wg,
					nil,
					&EndpointRet{value.data},
					value.input,
				)
				if errors.Is(err, flow.ErrStopGoroutine) {
					continue
				}

				if err != nil {
					t.Errorf("Script.Run() error = %v", err)
					return
				}
				if diff := deep.Equal(string(got.GetBinaryData()), string(tt.want.GetBinaryData())); diff != nil {
					t.Errorf("Script.Run() = %v", diff)
					return
				}
			}
		})
	}
}
