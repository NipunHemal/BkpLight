package state

import "time"

type JobRevision struct {
	ID, JobID, SourceID, ConfigID string
	Snapshot                      []byte
}
type Channel struct {
	Revision string
	Events   []string
}
type Run struct {
	ID, JobRevisionID, RequestID, PayloadHash, State string
	Version                                          int64
	StartedAt                                        time.Time
	ErrorCode                                        string
}
type Begin struct{ ID, JobRevisionID, RequestID, PayloadHash string }
type Transition struct {
	RunID, From, To string
	Version         int64
	ErrorCode       string
}
type Page struct {
	Items []Run
	Next  string
}
