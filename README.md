## Micro Go
Minimal core components of microservices.

### How to use it?
`go get github.com/lhdhtrc/micro-go`
```go
package main

import (
	"fmt"
	micro "github.com/lhdhtrc/micro-go/pkg"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	instance := micro.New(logger)
	
	// How do I start a grpc service? 
	instance.InstallServer(func(server *grpc.Server) {
        
	}, "127.0.0.1:8080")
}
```

### Finally
- If you feel good, click on star.
- If you have a good suggestion, please ask the issue.