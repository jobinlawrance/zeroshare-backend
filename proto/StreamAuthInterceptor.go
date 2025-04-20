package proto

import (
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func StreamAuthInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			log.Println("no metadata")
		}
		log.Println(md)
		return handler(srv, ss)
	}
}
