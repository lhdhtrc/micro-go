package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	micro "github.com/lhdhtrc/micro-go/pkg/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func NewRegister(client *clientv3.Client, meta *micro.Meta, config *micro.ServiceConf) (*RegisterInstance, error) {
	ctx, cancel := context.WithCancel(context.Background())

	instance := &RegisterInstance{
		ctx:  ctx,
		meta: meta,

		config: config,
		client: client,
		cancel: cancel,
	}
	err := instance.initLease()

	return instance, err
}

type RegisterInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *clientv3.Client
	lease  clientv3.LeaseID

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

	s.config.Kernel.Language = "Golang"

	service.Meta = s.meta
	service.Kernel = s.config.Kernel
	service.Network = s.config.Network
	service.LeaseId = int(s.lease)
	service.RunDate = time.Now().Format(time.DateTime)

	val, _ := json.Marshal(service)

	_, err := s.client.Put(ctx, fmt.Sprintf("%s/%s/%s/%d", s.config.Namespace, s.meta.Env, service.Meta.AppId, s.lease), string(val), clientv3.WithLease(s.lease))
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
