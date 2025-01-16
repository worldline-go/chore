package main

import (
	"context"
	"sync"

	"github.com/worldline-go/initializer"
	"github.com/worldline-go/logz"

	"github.com/worldline-go/chore/cmd/chore/args"
)

func main() {
	initializer.Init(
		run,
		initializer.WithInitLog(false),
		initializer.WithOptionsLogz(logz.WithCaller(false)),
	)
}

func run(ctx context.Context, wg *sync.WaitGroup) error {
	return args.Execute(ctx, wg) //nolint:wrapcheck // no need
}
