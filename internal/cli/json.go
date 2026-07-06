package cli

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/state"
)

var outputJSON bool

type errorOutput struct {
	Error           string            `json:"error"`
	Lock            *state.LockStatus `json:"lock,omitempty"`
	SuggestedAction string            `json:"suggestedAction,omitempty"`
}

func printJSON(cmd *cobra.Command, value any) error {
	return writeJSON(cmd.OutOrStdout(), value)
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func errorPayload(err error) errorOutput {
	payload := errorOutput{Error: err.Error()}
	var lockErr *state.LockError
	if errors.As(err, &lockErr) {
		status := lockErr.Status
		payload.Lock = &status
		if status.Stale {
			payload.SuggestedAction = "retry; Forest can clear stale same-host locks, or run forest doctor --fix if the lock persists"
		} else {
			payload.SuggestedAction = "wait for the active Forest process to finish, then retry"
		}
	}
	return payload
}
