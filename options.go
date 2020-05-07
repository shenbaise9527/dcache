package main

import "flag"

// Options 参数.
type Options struct {
	httpAddress string
	raftAddress string
	joinAddress string
}

// NewOptions 解析参数.
func NewOptions() *Options {
	// 从参数中获取.
	httpAddress := flag.String("http", "127.0.0.1:6380", "http address")
	raftAddress := flag.String("raft", "127.0.0.1:6381", "raft tcp address")
	joinAddress := flag.String("join", "", "join address for raft cluster")
	flag.Parse()
	op := &Options{
		httpAddress: *httpAddress,
		raftAddress: *raftAddress,
		joinAddress: *joinAddress,
	}

	return op
}
