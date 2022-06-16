package lang

import (
	"fmt"
	"log"
	"os"

	"bake/internal/lang/config"

	"github.com/hashicorp/hcl/v2"
)

func (t Task) dryPrune(state config.State) (shouldApply bool, reason string, diags hcl.Diagnostics) {
	if state.Flags.Force {
		return true, "force prunning is in effect", nil
	}

	if t.Creates == "" {
		return false, "nothing to prune", nil
	}

	stat, err := os.Stat(t.Creates)
	if err != nil {
		return false, fmt.Sprintf(`"%s" doesn't exist`, t.Creates), nil
	}

	return true, fmt.Sprintf(`will delete "%s"`, stat.Name()), nil
}

func (t *Task) prune(log *log.Logger) hcl.Diagnostics {
	err := os.RemoveAll(t.Creates)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "error pruning task " + t.GetName(),
			Detail:   err.Error(),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}
