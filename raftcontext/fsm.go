package raftcontext

import (
	"io"

	"github.com/hashicorp/raft"
	"github.com/shenbaise9527/dcache/cache"
)

// FSM 状态机.
type FSM struct {
	cache.Cache
}

// Apply 状态机处理日志.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	return f.Do(logEntry.Data)
}

// Snapshot 状态机生成快照.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	datas, err := f.Marshal()
	if err != nil {
		return nil, err
	}

	snap := &snapShot{datas}
	return snap, nil
}

// Restore 状态机应用快照.
func (f *FSM) Restore(serialzed io.ReadCloser) error {
	return f.UnMarshal(serialzed)
}
