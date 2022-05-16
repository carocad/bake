package worker

import (
	"context"

	"bake/internal/lang"
	"bake/internal/module/contextualize"
	"github.com/hashicorp/hcl/v2"
	"golang.org/x/sync/errgroup"
)

const Parallelism = 4

func DO(semaphore *contextualize.Semaphore, deps []lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	group, ctx := errgroup.WithContext(context.TODO())
	group.SetLimit(Parallelism)
	for _, dep := range deps {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if diags, ok := err.(hcl.Diagnostics); ok {
				return nil, diags
			}
			panic("unknown error: " + err.Error())

		case <-semaphore.WaitFor(dep):
			eval := semaphore.Context(dep)
			actions, diagnostics := dep.Decode(eval)
			if diagnostics.HasErrors() {
				return nil, diagnostics
			}

			if !dep.Path().HasPrefix(lang.DataPrefix) {
				semaphore.Extend(actions)
				continue
			}

			// we need to refresh before the next actions are loaded since
			// they depend on the data values
			for _, action := range actions {
				action := action
				group.Go(func() error {
					diags := action.Apply()
					if diags.HasErrors() {
						return diags
					}

					semaphore.Append(action)
					return nil
				})
			}
		}
	}

	err := group.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	return semaphore.GetActions(), nil
}
