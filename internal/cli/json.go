package cli

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/state"
)

// outputJSON is package-level state bound to the persistent --json flag each
// time newRootCommand runs. Production binds it once per process; tests that
// execute commands must reset it to false before each run and must not use
// t.Parallel() while sharing it.
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
		switch {
		case !status.Exists:
			payload.SuggestedAction = "retry; the lock was removed concurrently"
		case status.Stale:
			payload.SuggestedAction = "retry; Forest can clear stale same-host locks, or run forest doctor --fix if the lock persists"
		default:
			payload.SuggestedAction = "wait for the active Forest process to finish, then retry"
		}
	}
	return payload
}
