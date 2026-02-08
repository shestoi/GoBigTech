package observability

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// parseGRPCFullMethod splits "/package.Service/Method" into serviceName ("package.Service") and method ("Method").
func parseGRPCFullMethod(fullMethod string) (serviceName, method string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	if fullMethod == "" {
		return "", ""
	}
	idx := strings.LastIndex(fullMethod, "/")
	if idx < 0 {
		return fullMethod, ""
	}
	return fullMethod[:idx], fullMethod[idx+1:]
}

// GRPCUnaryServerInterceptor возвращает unary server interceptor: извлекает trace из metadata, создаёт span на RPC.
func GRPCUnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)
	prop := otel.GetTextMapPropagator()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = prop.Extract(ctx, NewMetadataCarrier(md))
		rpcService, rpcMethod := parseGRPCFullMethod(info.FullMethod)
		if rpcService == "" {
			rpcService = info.FullMethod
		}
		if rpcMethod == "" {
			rpcMethod = info.FullMethod
		}
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", rpcService),
				attribute.String("rpc.method", rpcMethod),
			),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(st.Code())))
			}
		}
		return resp, err
	}
}

// GRPCUnaryClientInterceptor возвращает unary client interceptor: создаёт span, инжектит trace в outgoing metadata.
func GRPCUnaryClientInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	tracer := otel.Tracer(serviceName)
	prop := otel.GetTextMapPropagator()
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		rpcService, rpcMethod := parseGRPCFullMethod(method)
		if rpcService == "" {
			rpcService = method
		}
		if rpcMethod == "" {
			rpcMethod = method
		}
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", rpcService),
				attribute.String("rpc.method", rpcMethod),
			),
		)
		defer span.End()

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}
		prop.Inject(ctx, NewMetadataCarrier(md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(st.Code())))
			}
		}
		return err
	}
}
