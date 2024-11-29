package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	micro "github.com/lhdhtrc/micro-go/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strings"
)

func NewDiscover(client *clientv3.Client, config *micro.ServiceConfig) *DiscoverInstance {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &DiscoverInstance{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		config:  config,
		service: make(micro.ServiceInstance),
	}

	instance.bootstrap()

	return instance
}

type DiscoverInstance struct {
	config *micro.ServiceConfig
	client *clientv3.Client

	ctx    context.Context
	cancel context.CancelFunc

	service micro.ServiceInstance
	log     func(level micro.LogLevel, message string)
}

func (s *DiscoverInstance) GetService(name string) ([]*micro.ServiceNode, error) {
	return s.service.GetNodes(name)
}

func (s *DiscoverInstance) Watcher() {
	wc := s.client.Watch(s.ctx, s.config.Namespace, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for v := range wc {
		for _, e := range v.Events {
			//adapter(e)
			fmt.Println(e)
		}
	}
}
func (s *DiscoverInstance) Unwatch() {
	s.cancel()
}

func (s *DiscoverInstance) bootstrap() {
	res, err := s.client.Get(s.ctx, s.config.Namespace, clientv3.WithPrefix())
	if err != nil {
		return
	}

	for _, item := range res.Kvs {
		var val micro.ServiceNode
		if err = json.Unmarshal(item.Value, &val); err == nil {
			key := strings.Replace(string(item.Key), fmt.Sprintf("/%d", item.Lease), "", 1)
			s.service[key] = append(s.service[key], &val)
		}
	}
}

func (s *DiscoverInstance) adapter(e *clientv3.Event) {

}
