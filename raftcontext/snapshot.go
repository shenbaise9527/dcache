package raftcontext

import "github.com/hashicorp/raft"

// snapShot 快照.
type snapShot struct {
	datas []byte
}

// Persist 快照持久化.
func (s *snapShot) Persist(sink raft.SnapshotSink) error {
	_, err := sink.Write(s.datas)
	if err == nil {
		err = sink.Close()
	}

	if err != nil {
		_ = sink.Cancel()
	}

	return err
}

// Release 持久化之后调用.
func (s *snapShot) Release() {
}
