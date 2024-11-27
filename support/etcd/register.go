package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	micro "github.com/lhdhtrc/micro-go/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func NewRegister(client *clientv3.Client, config *micro.ServiceConfig) (*RegisterInstance, error) {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &RegisterInstance{
		ctx:    ctx,
		cancel: cancel,
		config: config,
		client: client,
	}
	err := instance.initLease()

	return instance, err
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

func (s *RegisterInstance) Install(service *micro.ServiceNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	val, _ := json.Marshal(service)

	service.Lease = int(s.lease)
	service.AppId = s.config.AppId
	service.OuterNetIp = s.config.OuterNetIp
	service.InternalNetIp = s.config.InternalNetIp

	_, err := s.client.Put(ctx, fmt.Sprintf("/%s/%s/%d", s.config.Namespace, service.Name, s.lease), string(val), clientv3.WithLease(s.lease))
	return err
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
func (s *RegisterInstance) initLease() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	grant, err := s.client.Grant(ctx, int64(s.config.TTL))
	if err != nil {
		return err
	}
	s.lease = grant.ID

	return nil
}

// SustainLease 保持租约
func (s *RegisterInstance) SustainLease() {
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

		if err := s.initLease(); err != nil {
			s.retry()
		} else {
			if s.retryAfter != nil {
				s.retryAfter()
			}
		}
	}
}
