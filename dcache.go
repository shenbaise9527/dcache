package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shenbaise9527/dcache/cache"
	"github.com/shenbaise9527/dcache/logger"
	"github.com/shenbaise9527/dcache/raftcontext"
)

// HttpResult http接口的结果.
type HttpResult struct {
	RetCode int         `json:"retcode"`
	RetDesc string      `json:"retdesc"`
	Data    interface{} `json:"data"`
}

func main() {
	// 解析参数.
	op := NewOptions()

	// 初始化日志.
	err := logger.NewLogger("./logs", 6)
	if err != nil {
		fmt.Println(err.Error())

		return
	}

	// 生成缓存对象.
	dc := cache.NewCache()

	// 生成raft.
	raftctx, err := raftcontext.NewRaftContext(
		op.httpAddress, op.raftAddress, op.joinAddress, dc)
	if err != nil {
		logger.Errorf("init raft failed[%s].", err.Error())

		return
	}

	// 依赖gin提供http功能.
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 中间件.
	r.Use(GinLoggerMiddleware())
	r.Use(GinRecoveryMiddleware())

	// 提供的http接口.
	r.GET("get", func(ctx *gin.Context) {
		key := ctx.Query("key")
		ret := HttpResult{
			RetCode: http.StatusOK,
		}

		ret.Data, err = dc.Get(key)
		if err != nil {
			ret.RetCode = http.StatusInternalServerError
			ret.RetDesc = err.Error()
		}

		ctx.SecureJSON(ret.RetCode, ret)
	})

	r.GET("keys", func(ctx *gin.Context) {
		ret := HttpResult{
			RetCode: http.StatusOK,
			Data:    dc.Keys(),
		}

		ctx.SecureJSON(ret.RetCode, ret)
	})

	r.POST("set", func(ctx *gin.Context) {
		var cmd cache.Command
		ctx.ShouldBindJSON(&cmd)
		cmd.Op = cache.CmdOpSet
		err = raftctx.Apply(ctx.Request.RequestURI, cmd)
		ret := HttpResult{
			RetCode: http.StatusOK,
		}

		if err != nil {
			ret.RetCode = http.StatusInternalServerError
			ret.RetDesc = err.Error()
		}

		ctx.SecureJSON(ret.RetCode, ret)
	})

	r.POST("del", func(ctx *gin.Context) {
		var cmd cache.Command
		ctx.ShouldBindJSON(&cmd)
		cmd.Op = cache.CmdOpDel
		err = raftctx.Apply(ctx.Request.RequestURI, cmd)
		ret := HttpResult{
			RetCode: http.StatusOK,
		}

		if err != nil {
			ret.RetCode = http.StatusInternalServerError
			ret.RetDesc = err.Error()
		}

		ctx.SecureJSON(ret.RetCode, ret)
	})

	r.POST("join", func(ctx *gin.Context) {
		var key raftcontext.ClusterReq
		err := ctx.ShouldBindJSON(&key)
		var ret raftcontext.ClusterResult
		if err != nil {
			ret.RetCode = http.StatusInternalServerError
			ret.RetDesc = err.Error()
		} else {
			ret = raftctx.JoinHandler(key)
		}

		ctx.SecureJSON(ret.RetCode, ret)
	})

	srv := &http.Server{
		Handler:      r,
		Addr:         op.httpAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Errorf("start service failed[%s].", err.Error())
	}
}

// GinLoggerMiddleware gin日志插件.
func GinLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		end := time.Now()
		latency := end.Sub(start)
		path := c.Request.URL.RequestURI()
		clientip := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		logger.Infof("|%3d|%13v|%15s|%s %s",
			statusCode, latency, clientip, method, path)
	}
}

// GinRecoveryMiddleware gin崩溃插件.
func GinRecoveryMiddleware() gin.HandlerFunc {
	return gin.RecoveryWithWriter(logger.GetLoggerWriter())
}
