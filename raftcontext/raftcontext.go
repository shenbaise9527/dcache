package raftcontext

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/vic/pkg/errors"
	jsoniter "github.com/json-iterator/go"
	"github.com/shenbaise9527/dcache/cache"
	"github.com/shenbaise9527/dcache/logger"
)

var JoinError = errors.New("join raftcluster failed.")

// ClusterReq 加入集群请求.
type ClusterReq struct {
	HTTPAddress string `json:"http"`
	RaftAddress string `json:"raft"`
}

// ClusterResult 加入集群响应.
type ClusterResult struct {
	RetCode int               `json:"retcode"`
	RetDesc string            `json:"retdesc"`
	Datas   map[string]string `json:"datas"`
}

// RaftContext raft上下文结构.
type RaftContext struct {
	raft         *raft.Raft        // raft节点.
	notifyCh     chan bool         // raft状态变更通知chan.
	stopCh       chan struct{}     // 停止chan.
	httpAddr     string            // 服务提供的对外http地址.
	raftAddr     string            // raft节点地址.
	joinAddr     string            // 申请加入集群地址.
	clusterinfo  map[string]string // 集群信息,key=raft节点 value=http地址.
	clusterMutex sync.Mutex        // 锁.
}

// NewRaftContext 创建raft.
func NewRaftContext(httpAddress, raftAddress, joinAddress string, dc cache.Cache) (*RaftContext, error) {
	cnf := raft.DefaultConfig()
	cnf.LocalID = raft.ServerID(raftAddress)
	cnf.Logger = hclog.New(&hclog.LoggerOptions{
		Name:            "raft",
		Output:          logger.GetLoggerWriter(),
		Level:           hclog.DefaultLevel,
		IncludeLocation: true,
		TimeFormat:      "2006-01-02 15:04:05",
	})
	leaderNotify := make(chan bool, 1)
	cnf.NotifyCh = leaderNotify
	tranAddr, err := net.ResolveTCPAddr("tcp", raftAddress)
	if err != nil {
		return nil, err
	}

	netLogger := hclog.New(&hclog.LoggerOptions{
		Name:            "raft-net",
		Output:          logger.GetLoggerWriter(),
		Level:           hclog.DefaultLevel,
		IncludeLocation: true,
		TimeFormat:      "2006-01-02 15:04:05",
	})
	tran, err := raft.NewTCPTransportWithLogger(
		tranAddr.String(), tranAddr, 3, 10*time.Second, netLogger)
	if err != nil {
		return nil, err
	}

	fsm := &FSM{dc}
	snapShotStore := raft.NewInmemSnapshotStore()
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	raftNode, err := raft.NewRaft(cnf, fsm, logStore, stableStore, snapShotStore, tran)
	if err != nil {
		return nil, err
	}

	raftCtx := &RaftContext{
		raft:        raftNode,
		notifyCh:    leaderNotify,
		httpAddr:    httpAddress,
		raftAddr:    raftAddress,
		joinAddr:    joinAddress,
		clusterinfo: make(map[string]string),
	}

	raftCtx.clusterinfo[raftAddress] = httpAddress
	if len(joinAddress) > 0 {
		err = raftCtx.joinCluster()
		if err != nil {
			return nil, err
		}
	} else {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      cnf.LocalID,
					Address: tran.LocalAddr(),
				},
			},
		}

		raftCtx.raft.BootstrapCluster(configuration)
	}

	go raftCtx.notify()

	return raftCtx, nil
}

// Apply 应用.
func (rctx *RaftContext) Apply(uri string, cmd cache.Command) error {
	datas, err := jsoniter.Marshal(cmd)
	if err != nil {
		return err
	}

	if raft.Leader == rctx.raft.State() {
		ret := rctx.raft.Apply(datas, 5*time.Second)
		if err := ret.Error(); err != nil {
			logger.Errorf("apply failed[%s].", err.Error())

			return err
		}

		return nil
	}

	// 非leader节点,转发.
	leaderHttp, ok := rctx.getLeaderHttpAddr()
	if !ok {
		// 没有leader,返回错误.
		return raft.ErrNotLeader
	}

	recmd := cache.Command{
		Datas: cmd.Datas,
	}

	buf, err := jsoniter.Marshal(recmd)
	if err != nil {
		return err
	}

	_, err = httpRequest(leaderHttp, uri, buf)
	return err
}

// JoinHandler
func (rctx *RaftContext) JoinHandler(req ClusterReq) ClusterResult {
	ret := ClusterResult{
		RetCode: http.StatusOK,
	}

	// 判断当前是否为leader,如果是直接调用AddVoter,如果不是要进行跳转.
	if raft.Leader == rctx.raft.State() {
		future := rctx.raft.AddVoter(
			raft.ServerID(req.RaftAddress),
			raft.ServerAddress(req.RaftAddress), 0, 0)
		err := future.Error()
		if err != nil {
			logger.Errorf("join cluster failed[%s], serverid[%s]", err.Error(), req.RaftAddress)
			ret.RetCode = http.StatusInternalServerError
			ret.RetDesc = err.Error()

			return ret
		}

		// 构建集群信息.
		rctx.clusterMutex.Lock()
		defer rctx.clusterMutex.Unlock()
		rctx.clusterinfo[req.RaftAddress] = req.HTTPAddress
		ret.Datas = make(map[string]string, len(rctx.clusterinfo))
		for k, v := range rctx.clusterinfo {
			ret.Datas[k] = v
		}

		return ret
	}

	// 获取leader.
	leaderHttp, ok := rctx.getLeaderHttpAddr()
	if !ok {
		// 没有leader,返回错误.
		err := raft.ErrNotLeader
		ret.RetCode = http.StatusInternalServerError
		ret.RetDesc = err.Error()

		return ret
	}

	// 转发到leader服务上.
	buf, err := jsoniter.Marshal(req)
	if err != nil {
		ret.RetCode = http.StatusInternalServerError
		ret.RetDesc = err.Error()

		return ret
	}

	redirectRet, err := httpRequest(leaderHttp, "/join", buf)
	if err != nil {
		ret.RetCode = http.StatusInternalServerError
		ret.RetDesc = err.Error()

		return ret
	}

	err = jsoniter.Unmarshal(redirectRet, &ret)
	if err != nil {
		ret.RetCode = http.StatusInternalServerError
		ret.RetDesc = err.Error()

		return ret
	}

	return ret
}

func (rctx *RaftContext) joinCluster() error {
	req := ClusterReq{
		HTTPAddress: rctx.httpAddr,
		RaftAddress: rctx.raftAddr,
	}

	buf, err := jsoniter.Marshal(req)
	if err != nil {
		return err
	}

	redirectRet, err := httpRequest(rctx.joinAddr, "/join", buf)
	if err != nil {
		return err
	}

	var ret ClusterResult
	err = jsoniter.Unmarshal(redirectRet, &ret)
	if err != nil {
		return err
	}

	if http.StatusOK != ret.RetCode {
		return JoinError
	}

	rctx.clusterMutex.Lock()
	defer rctx.clusterMutex.Unlock()
	rctx.clusterinfo = ret.Datas

	return nil
}

func (rctx *RaftContext) notify() {
	for {
		select {
		case leader := <-rctx.notifyCh:
			if leader {
				logger.Debugf("become leader")
			} else {
				logger.Debugf("become follower")
			}
		case <-rctx.stopCh:
			logger.Debugf("stop")
			return
		}
	}
}

func (rctx *RaftContext) getLeaderHttpAddr() (string, bool) {
	// 获取leader.
	server := rctx.raft.Leader()
	if len(server) == 0 {
		// 没有leader.
		return "", false
	}

	// 获取leader的http接口.
	rctx.clusterMutex.Lock()
	leaderHttp, ok := rctx.clusterinfo[string(server)]
	rctx.clusterMutex.Unlock()
	if !ok {
		// 不存在,返回错误.
		return "", false
	}

	return leaderHttp, true
}

func httpRequest(httpAddr, uri string, datas []byte) ([]byte, error) {
	// 加入集群中.
	cli := &http.Client{
		Timeout: time.Second * 5,
	}

	if !strings.HasPrefix(httpAddr, "http") {
		httpAddr = "http://" + httpAddr
	}

	url := httpAddr + uri
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(datas))
	if err != nil {
		return nil, err
	}

	rsp, err := cli.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if http.StatusOK != rsp.StatusCode {
		return nil, JoinError
	}

	defer rsp.Body.Close()
	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	return buf, err
}
