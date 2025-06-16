package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lhdhtrc/func-go/array"
	micro "github.com/lhdhtrc/micro-go/pkg/core"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewDiscover(client *clientv3.Client, meta *micro.Meta, config *micro.ServiceConf) (*DiscoverInstance, error) {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &DiscoverInstance{
		ctx:  ctx,
		meta: meta,

		cancel:  cancel,
		client:  client,
		config:  config,
		methods: make(micro.ServiceMethods),
		service: make(micro.ServiceDiscover),
	}
	err := instance.bootstrap()

	return instance, err
}

type DiscoverInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *clientv3.Client

	ctx    context.Context
	cancel context.CancelFunc

	log func(level micro.LogLevel, message string)

	methods micro.ServiceMethods
	service micro.ServiceDiscover
}

// GetService 获取服务
func (s *DiscoverInstance) GetService(sm string) ([]*micro.ServiceNode, error) {
	appId, err := s.methods.GetAppId(sm)
	if err != nil {
		return nil, err
	}
	return s.service.GetNodes(appId)
}

// Watcher 服务发现
func (s *DiscoverInstance) Watcher() {
	wc := s.client.Watch(s.ctx, fmt.Sprintf("/%s/%s", s.config.Namespace, s.meta.Env), clientv3.WithPrefix(), clientv3.WithPrevKV())
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
			s.service[val.Meta.AppId] = append(s.service[val.Meta.AppId], &val)
			val.ParseMethod(s.methods)
		}
	}

	return nil
}

// adapter 服务发现适配器
func (s *DiscoverInstance) adapter(e *clientv3.Event) {
	var (
		tv []byte
	)

	if e.PrevKv != nil {
		tv = e.PrevKv.Value
	} else {
		tv = e.Kv.Value
	}

	var val micro.ServiceNode
	if err := json.Unmarshal(tv, &val); err != nil {
		if s.log != nil {
			s.log(micro.Error, err.Error())
		}
		return
	}
	val.ParseMethod(s.methods)

	key := val.Meta.AppId

	switch e.Type {
	// PUT，新增或替换
	case clientv3.EventTypePut:
		s.service[key] = append(s.service[key], &val)
	// DELETE
	case clientv3.EventTypeDelete:
		s.service[key] = array.Filter(s.service[key], func(index int, item *micro.ServiceNode) bool {
			return item.LeaseId != val.LeaseId
		})
	}
}
