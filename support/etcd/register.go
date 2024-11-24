package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	micro "github.com/lhdhtrc/micro-go/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func NewRegister(client *clientv3.Client, config *micro.ServiceConfig) *RegisterInstance {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &RegisterInstance{
		ctx:    ctx,
		cancel: cancel,
		config: config,
		client: client,
	}

	instance.initLease()

	return instance
}

type RegisterInstance struct {
	config *micro.ServiceConfig
	client *clientv3.Client
	lease  clientv3.LeaseID
	kv     clientv3.KV

	ctx    context.Context
	cancel context.CancelFunc

	retryCount  uint32
	retryBefore func()
	retryAfter  func()
	log         func(level micro.LogLevel, message string)
}

func (s *RegisterInstance) Install(service *micro.ServiceNode) {
	val, _ := json.Marshal(service)

	service.Lease = int(s.lease)
	service.AppId = s.config.AppId
	service.OuterNetIp = s.config.OuterNetIp
	service.InternalNetIp = s.config.InternalNetIp

	_, _ = s.client.Put(context.Background(), fmt.Sprintf("/%s/%s/%d", s.config.Namespace, service.Name, s.lease), string(val), clientv3.WithLease(s.lease))
}
func (s *RegisterInstance) Uninstall() {
	defer s.cancel()
	_, _ = s.client.Revoke(context.Background(), s.lease)
	return
}

// WithLog 日志记录
func (s *RegisterInstance) WithLog(handle func(level micro.LogLevel, message string)) {
	s.log = handle
}

// WithRetryBefore 重试之前执行
func (s *RegisterInstance) WithRetryBefore(handle func()) {
	s.retryBefore = handle
}

// WithRetryAfter 重试之后执行
func (s *RegisterInstance) WithRetryAfter(handle func()) {
	s.retryAfter = handle
}

// initLease 初始化租约
func (s *RegisterInstance) initLease() {
	grant, err := s.client.Grant(s.ctx, int64(s.config.TTL))
	if err != nil {
		s.log(micro.Error, err.Error())
		s.retry()
		return
	}
	s.lease = grant.ID
	go s.sustainLease()
}

// sustainLease 保持租约
func (s *RegisterInstance) sustainLease() {
	lease, _ := s.client.KeepAlive(s.ctx, s.lease)

	for {
		select {
		case <-s.ctx.Done():
			return
		case _, ok := <-lease:
			if !ok {
				s.retry()
				return
			}
			if s.retryCount != 0 {
				s.retryCount = 0
			}
		}
	}
}

// retry 服务重试
func (s *RegisterInstance) retry() {
	if s.retryCount < s.config.MaxRetry {
		if s.retryBefore != nil {
			s.retryBefore()
		}
		time.Sleep(5 * time.Second)

		s.retryCount++
		if s.log != nil {
			s.log(micro.Info, fmt.Sprintf("etcd retry lease: %d/%d", s.retryCount, s.config.MaxRetry))
		}

		s.initLease()

		if s.retryAfter != nil {
			s.retryAfter()
		}
	}
}
