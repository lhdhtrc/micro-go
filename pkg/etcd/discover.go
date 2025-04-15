package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lhdhtrc/func-go/array"
	micro "github.com/lhdhtrc/micro-go/pkg/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strings"
)

func NewDiscover(client *clientv3.Client, config *micro.ServiceConf) (*DiscoverInstance, error) {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &DiscoverInstance{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		config:  config,
		service: make(micro.ServiceInstance),
	}
	err := instance.bootstrap()

	return instance, err
}

type DiscoverInstance struct {
	config *micro.ServiceConf
	client *clientv3.Client

	ctx    context.Context
	cancel context.CancelFunc

	service micro.ServiceInstance
	log     func(level micro.LogLevel, message string)
}

// GetService 获取服务
func (s *DiscoverInstance) GetService(name string) ([]*micro.ServiceNode, error) {
	return s.service.GetNodes(name)
}

// Watcher 服务发现
func (s *DiscoverInstance) Watcher() {
	wc := s.client.Watch(s.ctx, s.config.Namespace, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for v := range wc {
		for _, e := range v.Events {
			s.adapter(e)
		}
	}
}

// Unwatch 释放资源
func (s *DiscoverInstance) Unwatch() {
	s.cancel()
}

// WithLog 日志记录
func (s *DiscoverInstance) WithLog(handle func(level micro.LogLevel, message string)) {
	s.log = handle
}

// bootstrap 初始化引导
func (s *DiscoverInstance) bootstrap() error {
	res, err := s.client.Get(s.ctx, s.config.Namespace, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	for _, item := range res.Kvs {
		var val micro.ServiceNode
		if err = json.Unmarshal(item.Value, &val); err == nil {
			key := strings.Replace(string(item.Key), fmt.Sprintf("/%d", item.Lease), "", 1)
			s.service[key] = append(s.service[key], &val)
		}
	}

	return nil
}

// adapter 服务发现适配器
func (s *DiscoverInstance) adapter(e *clientv3.Event) {
	var (
		key   string
		tv    []byte
		lease int64
	)

	if e.PrevKv != nil {
		key = string(e.PrevKv.Key)
		tv = e.PrevKv.Value
		lease = e.PrevKv.Lease
	} else {
		key = string(e.Kv.Key)
		tv = e.Kv.Value
		lease = e.Kv.Lease
	}

	key = strings.Replace(key, fmt.Sprintf("/%d", lease), "", 1)
	var val micro.ServiceNode
	if err := json.Unmarshal(tv, &val); err != nil {
		if s.log != nil {
			s.log(micro.Error, err.Error())
		}
		return
	}

	switch e.Type {
	// PUT，新增或替换
	case clientv3.EventTypePut:
		s.service[key] = append(s.service[key], &val)
	// DELETE
	case clientv3.EventTypeDelete:
		s.service[key] = array.Filter(s.service[key], func(index int, item *micro.ServiceNode) bool {
			return item.Lease != val.Lease
		})
	}
}
