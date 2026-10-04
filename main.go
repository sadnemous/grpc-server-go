package main

import (
	"context"
	pb "grpc-server-go/grpc"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
	// Share the service implementation between gRPC and REST.
	server := &Server{}

	// Set up the existing gRPC server.
	lis, err := net.Listen("tcp", ":44444")
	if err != nil {
		log.Fatal(err)
	}

	grpcSrv := grpc.NewServer()
	pb.RegisterGreeterServer(grpcSrv, server)

	// Create the REST gateway and register its generated handlers.
	mux := runtime.NewServeMux()
	err = pb.RegisterGreeterHandlerServer(
		context.Background(),
		mux,
		server,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Serve gRPC concurrently because Serve blocks.
	go func() {
		log.Println("gRPC server listening on :44444")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	// Serve REST on the main goroutine.
	log.Println("REST server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
