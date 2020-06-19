package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type cacheManager struct {
	data map[string]string
	sync.RWMutex
}

func NewCacheManager() *cacheManager {
	cm := &cacheManager{}
	cm.data = make(map[string]string)
	return cm
}

func (c *cacheManager) Get(key string) string {
	c.RLock()
	defer c.RUnlock()
	ret := c.data[key]
	return ret
}

func (c *cacheManager) GetKey(key string) (string,error) {
	c.RLock()
	defer c.RUnlock()
	if value,ok := c.data[key];ok {
		return value,nil
	}
	return "",errors.New(fmt.Sprintf("key %s not found",key))
}

func (c *cacheManager) Set(key string, value string) error {
	c.Lock()
	defer c.Unlock()
	c.data[key] = value
	return nil
}

func (c *cacheManager) Del(key string) error {
	c.RLock()
	defer c.RUnlock()
	delete(c.data,key)
	return nil
}

// Marshal serializes cache data
func (c *cacheManager) Marshal() ([]byte, error) {
	c.RLock()
	defer c.RUnlock()
	dataBytes, err := json.Marshal(c.data)
	return dataBytes, err
}

func (c *cacheManager) MarshalForKey(key string) ([]byte, error) {
	c.RLock()
	defer c.RUnlock()
	if value,ok := c.data[key];ok {
		dataBytes, err := json.Marshal(map[string]string{key:value})
		return dataBytes, err
	}
	return []byte{},errors.New(fmt.Sprintf("key %s not found",key))
}

// UnMarshal deserializes cache data
func (c *cacheManager) UnMarshal(serialized io.ReadCloser) error {
	var newData map[string]string
	if err := json.NewDecoder(serialized).Decode(&newData); err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()
	c.data = newData

	return nil
}

// copy on write
func (c *cacheManager) Clone() map[string]string {
	c.RLock()
	defer c.RUnlock()
	result := make(map[string]string)
	for k,v := range c.data {
		result[k] = v
	}
	return result
}

func (c *cacheManager) Len() int64 {
	return int64(len(c.data))
}
