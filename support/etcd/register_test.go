package etcd

import (
	"fmt"
	micro "github.com/lhdhtrc/micro-go/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"runtime"
	"testing"
)

func TestRegister(t *testing.T) {
	// routine 2
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"192.168.1.100:10206"},
		Username:  "root",
		Password:  "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	// routine 8

	// 创建一个服务配置
	config := &micro.ServiceConfig{
		AppId:         "test-app",
		OuterNetIp:    "127.0.0.1",
		InternalNetIp: "127.0.0.1",
		Namespace:     "test-namespace",
		TTL:           10,
		MaxRetry:      3,
	}

	// 创建一个服务节点
	service := &micro.ServiceNode{
		Name: "test-service",
	}

	// 初始化注册实例
	reg, err := NewRegister(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// routine 7

	// 服务重试之前（如果不成功则会继续执行该函数）
	reg.WithRetryBefore(func() {
		fmt.Println("重试之前", runtime.NumGoroutine())
	})
	// 服务重试成功之后
	reg.WithRetryAfter(func() {
		// 将节点信息重新注册到注册中心
		_ = reg.Install(service)
		// 重新续约
		go reg.SustainLease()

		fmt.Println("重试之后", runtime.NumGoroutine())
	})
	// 使用日志
	reg.WithLog(func(level micro.LogLevel, message string) {
		fmt.Println("log", level, message)
	})

	// 将服务节点信息注册到注册中心
	if err := reg.Install(service); err != nil {
		fmt.Println("install")
		return
	}
	// routine 7

	// 续约保活，在lease失效后会移除服务节点信息
	go reg.SustainLease()
	// routine 8

	fmt.Println("测试完毕")

	// 非必须
	for {

	}
}
