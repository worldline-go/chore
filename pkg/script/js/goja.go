package js

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/rs/zerolog/log"
	"github.com/worldline-go/chore/pkg/transfer"
)

var ErrThrow = errors.New("throw")

type Goja struct {
	runtime   *goja.Runtime
	DataName  string
	functions map[string]interface{}
}

func NewGoja() Goja {
	return Goja{
		runtime:   goja.New(),
		DataName:  "data",
		functions: map[string]interface{}{},
	}
}

func (g *Goja) SetData(data interface{}) error {
	return g.runtime.Set(g.DataName, data)
}

func (g *Goja) Set(name string, data interface{}) error {
	return g.runtime.Set(name, data)
}

func (g *Goja) SetFunction(name string, fn interface{}) {
	g.functions[name] = fn
}

func (g *Goja) RunString(value string) (goja.Value, error) {
	return g.runtime.RunString(value)
}

func (g *Goja) RunScript(ctx context.Context, script string, inputs []interface{}) ([]byte, error) {
	if _, err := g.runtime.RunString(script); err != nil {
		return nil, fmt.Errorf("script cannot read: %w", err)
	}

	mainScript, ok := goja.AssertFunction(g.runtime.Get("main"))
	if !ok {
		return nil, fmt.Errorf("main function not found")
	}

	// set script special functions
	if err := setScriptFuncs(ctx, g.runtime, g.functions); err != nil {
		return nil, err
	}

	passValues := []goja.Value{}
	for i := range inputs {
		passValues = append(passValues, g.runtime.ToValue(inputs[i]))
	}

	res, err := mainScript(goja.Undefined(), passValues...)
	if err != nil {
		var jserrException *goja.Exception

		if errors.As(err, &jserrException) {
			retVal := jserrException.Value().Export()

			if strings.HasPrefix(err.Error(), "ReferenceError: ") && !strings.HasPrefix(fmt.Sprint(retVal), "ReferenceError: ") {
				log.Ctx(ctx).Error().Msgf("main function run: %v", err)

				return []byte(err.Error()), fmt.Errorf("main function run: %w", err)
			}

			return transfer.DataToBytes(retVal), ErrThrow
		}

		return nil, fmt.Errorf("main function run: %w", err)
	}

	return transfer.DataToBytes(res.Export()), nil
}
