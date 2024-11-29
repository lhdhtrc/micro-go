package etcd

import (
	"context"
	"fmt"
	micro "github.com/lhdhtrc/micro-go/core"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewDiscover() *DiscoverInstance {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &DiscoverInstance{
		ctx:    ctx,
		cancel: cancel,
	}

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	core.Find(prefix, init)
	wc := s.client.Watch(ctx, s.config.Namespace, clientv3.WithPrefix(), clientv3.WithPrevKV())
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
