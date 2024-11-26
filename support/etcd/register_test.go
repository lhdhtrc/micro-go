package etcd

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"testing"
	"time"

	micro "github.com/lhdhtrc/micro-go/core"
)

func TestRegister(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// 创建一个服务配置
	config := &micro.ServiceConfig{
		AppId:         "test-app",
		OuterNetIp:    "127.0.0.1",
		InternalNetIp: "127.0.0.1",
		Namespace:     "test-namespace",
		TTL:           10,
		MaxRetry:      3,
	}

	// todo 测试

	// 创建一个服务节点
	service := &micro.ServiceNode{
		Name: "test-service",
	}

	// 初始化注册实例
	reg := NewRegister(cli, config)

	// 安装服务
	reg.Install(service)

	// 验证服务是否注册成功
	//expectedKey := fmt.Sprintf("/%s/%s/%d", config.Namespace, service.Name, register.lease)
	//expectedVal, _ := json.Marshal(service)
	//actualVal, err := mockClient.KV.Get(context.Background(), expectedKey)
	//require.NoError(t, err)
	//assert.Equal(t, 1, len(actualVal.Kvs))
	//assert.Equal(t, expectedKey, string(actualVal.Kvs[0].Key))
	//assert.Equal(t, string(expectedVal), string(actualVal.Kvs[0].Value))
}
