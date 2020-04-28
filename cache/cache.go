package cache

import (
	"io"

	"github.com/pkg/errors"
)

var (
	CacheNilError          = errors.New("Key is not exist")
	CacheNotSupportCommand = errors.New("dont support command")
	CacheParamError        = errors.New("param error")
)

// Cache 缓存接口.
type Cache interface {
	// Get 获取指定键值.
	Get(string) (string, error)

	// Marshal 编码.
	Marshal() ([]byte, error)

	// UnMarshal 解码.
	UnMarshal(io.ReadCloser) error

	// Do 执行命令.
	Do([]byte) error
}

// CmdOp 操作类型.
type CmdOp uint8

const (
	// CmdOpSet 设置命令.
	CmdOpSet CmdOp = iota

	// CmdOpDel 删除命令.
	CmdOpDel
)

// Command 命令.
type Command struct {
	Op    CmdOp
	Datas []string
}
