package etcd

import (
	"fmt"
	micro "github.com/lhdhtrc/micro-go/pkg/core"
	clientv3 "go.etcd.io/etcd/client/v3"
	"testing"
)

func TestDiscover(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"192.168.1.100:10206"},
		Username:  "root",
		Password:  "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// 创建一个服务配置
	config := &micro.ServiceConf{
		Network: &micro.Network{
			SN:       "xxxx",
			Internal: "192.168.1.100",
			External: "192.168.1.100",
		},
		Namespace: "test-namespace",
		TTL:       10,
		MaxRetry:  3,
	}

	// 初始服务发现实例
	dis, err := NewDiscover(cli, &micro.Meta{
		AppId:   "test-service",
		Env:     "prod",
		Version: "v0.0.1",
	}, config)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 使用日志, 非必要不要启用
	dis.WithLog(func(level micro.LogLevel, message string) {
		fmt.Println("log", level, message)
	})

	go dis.Watcher()

	fmt.Println("测试完毕")

	// 非必须
	for {

	}
}
