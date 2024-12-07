package proto

import (
	"log"
	"net"
	pb "zeroshare-backend/proto/sse"
	"zeroshare-backend/structs"

	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type server struct {
	pb.UnimplementedDeviceServiceServer
	DB  *gorm.DB // Add your DB connection
	log *zap.Logger
}

func (s *server) DeviceStream(stream pb.DeviceService_DeviceStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			s.log.Fatal("Error receiving request:", zap.Error(err))
			return err
		}

		dataMap := req.GetData().AsMap() // Convert Struct to map

		s.log.Debug("Received request",
			zap.String("type", req.Type),
			zap.String("unique_id", req.UniqueId),
			zap.Any("data", dataMap),
		)

		device := &structs.Device{}
		result := s.DB.Where("device_id = ?", req.UniqueId).First(&device)
		if result.Error != nil {
			s.log.Error("Error fetching device from database:", zap.Error(result.Error))
			return result.Error
		}

		// Business logic: process the request
		response := &pb.SSEResponse{
			Type: req.Type,
			Data: req.Data,
			Device: &pb.Device{
				Id:          device.ID.String(),
				MachineName: device.MachineName,
				Platform:    device.Platform,
				DeviceId:    device.DeviceId,
				IpAddress:   device.IpAddress,
				Created:     device.Created,
				Updated:     device.Updated,
				UserId:      device.UserId.String(),
			},
		}

		// Send the response back to the client
		if err := stream.Send(response); err != nil {
			s.log.Error("Error sending response:", zap.Error(err))
			return err
		}
	}
}

func StartGRPCServer(db *gorm.DB) {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	zapLogger, _ := zap.NewDevelopment()

	// Shared options for the logger, with a custom gRPC code to log level function.
	opts := []grpc_zap.Option{
		grpc_zap.WithLevels(grpc_zap.DefaultCodeToLevel),
	}
	// Make sure that log statements internal to gRPC library are logged using the logrus Logger as well.
	grpc_zap.ReplaceGrpcLoggerV2(zapLogger)

	grpcServer := grpc.NewServer(
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.StreamServerInterceptor(zapLogger, opts...),
		),
		grpc.ChainUnaryInterceptor(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.UnaryServerInterceptor(zapLogger, opts...),
		),
		grpc.StreamInterceptor(StreamAuthInterceptor()), // Register streaming interceptor
	)
	pb.RegisterDeviceServiceServer(grpcServer, &server{DB: db, log: zapLogger})

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
