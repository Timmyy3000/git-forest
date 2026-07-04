package state

import "time"

type Store struct {
	Version     int        `json:"version"`
	RepoRoot    string     `json:"repoRoot"`
	DefaultBase string     `json:"defaultBase"`
	Worktrees   []Worktree `json:"worktrees"`
}

type Worktree struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Branch    string    `json:"branch"`
	Path      string    `json:"path"`
	Base      string    `json:"base"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy Actor     `json:"createdBy"`
	Activity  Activity  `json:"activity"`
	Status    Status    `json:"status"`
}

type Actor struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
}

type Activity struct {
	Phase      string    `json:"phase"`
	Agent      string    `json:"agent,omitempty"`
	Note       string    `json:"note,omitempty"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

type Status struct {
	LastKnown     string    `json:"lastKnown"`
	LastCheckedAt time.Time `json:"lastCheckedAt"`
}

type Event struct {
	Time   time.Time `json:"time"`
	Type   string    `json:"type"`
	ID     string    `json:"id"`
	Detail string    `json:"detail,omitempty"`
}

func NewStore(root string) Store {
	return Store{Version: 1, RepoRoot: root, DefaultBase: "main", Worktrees: []Worktree{}}
}

func (s Store) Find(id string) (Worktree, int, bool) {
	for i, wt := range s.Worktrees {
		if wt.ID == id || wt.Name == id || wt.Branch == id {
			return wt, i, true
		}
	}
	return Worktree{}, -1, false
}
