package cache

import (
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/shenbaise9527/dcache/logger"
)

// cacheimpl 缓存结构.
type cacheimpl struct {
	datas map[string]string
	mutex sync.RWMutex
}

// NewCache 创建缓存对象.
func NewCache() Cache {
	return &cacheimpl{
		datas: make(map[string]string, 1024),
	}
}

// Get 查询.
func (c *cacheimpl) Get(key string) (string, error) {
	c.rlock()
	defer c.runlock()
	val, ok := c.datas[key]
	if ok {
		return val, nil
	}

	return "", CacheNilError
}

// Marshal 编码.
func (c *cacheimpl) Marshal() ([]byte, error) {
	c.lock()
	defer c.unlock()
	return jsoniter.Marshal(c.datas)
}

// UnMarshal 解码.
func (c *cacheimpl) UnMarshal(serialized io.ReadCloser) error {
	defer serialized.Close()
	var newDatas map[string]string
	err := jsoniter.NewDecoder(serialized).Decode(&newDatas)
	if err != nil {
		return err
	}

	c.lock()
	defer c.unlock()
	c.datas = newDatas

	return nil
}

// Apply 应用.
func (c *cacheimpl) Do(datas []byte) error {
	e := Command{}
	err := jsoniter.Unmarshal(datas, &e)
	if err != nil {
		logger.Errorf("Failed to unmarshal raft logentry[%s].", err.Error())

		return err
	}

	if CmdOpSet == e.Op {
		sz := len(e.Datas)
		c.lock()
		defer c.unlock()
		for i := 0; i < sz; i++ {
			j := i + 1
			if j < sz {
				c.datas[e.Datas[i]] = e.Datas[j]
			} else {
				c.datas[e.Datas[i]] = ""
			}

			i++
		}

		return nil
	} else if CmdOpDel == e.Op {
		sz := len(e.Datas)
		c.lock()
		defer c.unlock()
		for i := 0; i < sz; i++ {
			delete(c.datas, e.Datas[i])
		}

		return nil
	}

	return CacheNotSupportCommand
}

func (c *cacheimpl) rlock() {
	c.mutex.RLock()
}

func (c *cacheimpl) runlock() {
	c.mutex.RUnlock()
}

func (c *cacheimpl) lock() {
	c.mutex.Lock()
}

func (c *cacheimpl) unlock() {
	c.mutex.Unlock()
}
