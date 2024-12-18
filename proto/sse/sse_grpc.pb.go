// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.0
// source: sse.proto

package sse

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	DeviceService_DeviceStream_FullMethodName = "/sse.DeviceService/DeviceStream"
)

// DeviceServiceClient is the client API for DeviceService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DeviceServiceClient interface {
	DeviceStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[SSERequest, SSEResponse], error)
}

type deviceServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDeviceServiceClient(cc grpc.ClientConnInterface) DeviceServiceClient {
	return &deviceServiceClient{cc}
}

func (c *deviceServiceClient) DeviceStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[SSERequest, SSEResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &DeviceService_ServiceDesc.Streams[0], DeviceService_DeviceStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SSERequest, SSEResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DeviceService_DeviceStreamClient = grpc.BidiStreamingClient[SSERequest, SSEResponse]

// DeviceServiceServer is the server API for DeviceService service.
// All implementations must embed UnimplementedDeviceServiceServer
// for forward compatibility.
type DeviceServiceServer interface {
	DeviceStream(grpc.BidiStreamingServer[SSERequest, SSEResponse]) error
	mustEmbedUnimplementedDeviceServiceServer()
}

// UnimplementedDeviceServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedDeviceServiceServer struct{}

func (UnimplementedDeviceServiceServer) DeviceStream(grpc.BidiStreamingServer[SSERequest, SSEResponse]) error {
	return status.Errorf(codes.Unimplemented, "method DeviceStream not implemented")
}
func (UnimplementedDeviceServiceServer) mustEmbedUnimplementedDeviceServiceServer() {}
func (UnimplementedDeviceServiceServer) testEmbeddedByValue()                       {}

// UnsafeDeviceServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DeviceServiceServer will
// result in compilation errors.
type UnsafeDeviceServiceServer interface {
	mustEmbedUnimplementedDeviceServiceServer()
}

func RegisterDeviceServiceServer(s grpc.ServiceRegistrar, srv DeviceServiceServer) {
	// If the following call pancis, it indicates UnimplementedDeviceServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&DeviceService_ServiceDesc, srv)
}

func _DeviceService_DeviceStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DeviceServiceServer).DeviceStream(&grpc.GenericServerStream[SSERequest, SSEResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DeviceService_DeviceStreamServer = grpc.BidiStreamingServer[SSERequest, SSEResponse]

// DeviceService_ServiceDesc is the grpc.ServiceDesc for DeviceService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var DeviceService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sse.DeviceService",
	HandlerType: (*DeviceServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "DeviceStream",
			Handler:       _DeviceService_DeviceStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "sse.proto",
}
