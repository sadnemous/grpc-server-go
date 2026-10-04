package main

import (
	"context"
	pb "grpc-server-go/grpc"
	"log"
	"net"

	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedGreeterServer
}

func (s *Server) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	response := &pb.HelloResponse{
		OutMessage: "Hello " + req.GetInpName(),
	}
	return response, nil
}

func main() {
	lis, err := net.Listen("tcp", ":44444")
	if err != nil {
		panic(err)
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterGreeterServer(grpcSrv, &Server{})
	log.Printf("gRPC server start on port - 44444")
	grpcSrv.Serve(lis)
}
