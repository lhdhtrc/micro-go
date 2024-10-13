package micro

import (
	"fmt"
	"google.golang.org/grpc"
	"net"
)

func (core *CoreEntity) InstallServer(handle func(server *grpc.Server), address string, options ...grpc.ServerOption) {
	logPrefix := "install grpc server"
	core.logger.Info(fmt.Sprintf("%s %s %s", logPrefix, address, "start ->"))

	listen, err := net.Listen("tcp", address)
	if err != nil {
		core.logger.Error(fmt.Sprintf("%s %s", logPrefix, err.Error()))
		return
	}
	server := grpc.NewServer(options...)

	/*-------------------------------------Register Microservice---------------------------------*/
	if handle != nil {
		handle(server)
	}
	/*-------------------------------------Register Microservice---------------------------------*/

	core.logger.Info(fmt.Sprintf("%s %s", logPrefix, "register server done ->"))
	go func() {
		sErr := server.Serve(listen)
		if sErr != nil {
			core.logger.Error(fmt.Sprintf("%s %s", logPrefix, sErr.Error()))
			return
		}
	}()

	core.server = server
}

func (core *CoreEntity) UninstallServer() {
	core.server.Stop()
}
