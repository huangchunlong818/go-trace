package traces

import (
	"context"
	"errors"
	"github.com/aliyun-sls/opentelemetry-go-provider-sls/provider"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// 初始化trace
func InitTrace(config TraceConfig) (trace.TracerProvider, propagation.TextMapPropagator, error) {
	slsConfig, err := provider.NewConfig(provider.WithServiceName(config.ServiceName),
		provider.WithServiceNamespace(config.ServiceNamespace),
		provider.WithServiceVersion(config.Version),
		provider.WithTraceExporterEndpoint(config.Endpoint),
		provider.WithMetricExporterEndpoint(config.Endpoint),
		provider.WithSLSConfig(config.Project, config.Instance, config.AccessKeyId, config.AccessKeySecret))
	if err != nil {
		return nil, nil, err
	}
	if err = provider.Start(slsConfig); err != nil {
		return nil, nil, err
	}

	//defer provider.Shutdown(slsConfig)
	tracerProvider := otel.GetTracerProvider()
	propagator := otel.GetTextMapPropagator()

	return tracerProvider, propagator, nil
}

type TraceConfig struct {
	Version          string
	Endpoint         string
	Project          string
	Instance         string
	AccessKeyId      string
	AccessKeySecret  string
	ServiceName      string
	ServiceNamespace string
}

// tracer span
type TracerSpan struct {
	config     TraceConfig
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
	tracer     trace.Tracer
}

var newTrace *TracerSpan

func GetNewTracerSpan(config TraceConfig) (*TracerSpan, error) {
	if newTrace == nil {
		providers, propagator, err := InitTrace(config)
		if err != nil {
			return nil, err
		}
		newTrace = &TracerSpan{
			config:     config,
			provider:   providers,
			propagator: propagator,
			tracer:     GetTracer(providers, propagator, config.ServiceName),
		}
	}
	return newTrace, nil
}

// 获取provider
func (tc *TracerSpan) GetProvider() trace.TracerProvider {
	return tc.provider
}

// 获取propagator
func (tc *TracerSpan) GetPropagator() propagation.TextMapPropagator {
	return tc.propagator
}

// 获取tracer
func (tc *TracerSpan) GetTracer() trace.Tracer {
	return tc.tracer
}

// 获取tracer
func GetTracer(provider trace.TracerProvider, propagator propagation.TextMapPropagator, name string) trace.Tracer {
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagator)
	return otel.Tracer(name)
}

// 获取Span
func (tc *TracerSpan) StartSpan(ctx context.Context, spanName string, types string) (context.Context, trace.Span, error) {
	var spanKind trace.SpanKind
	switch types {
	case "server":
		spanKind = trace.SpanKindServer
	case "client":
		spanKind = trace.SpanKindClient
	case "rabbitmq", "cron":
		spanKind = trace.SpanKindInternal
	default:
		return nil, nil, errors.New("trace 启动方式错误，只支持服务端和客户端以及内部方式")
	}

	ctx, span := tc.tracer.Start(ctx, spanName, trace.WithSpanKind(spanKind)) //trace.WithAttributes  trace.WithTimestamp   trace.WithLinks  trace.WithNewRoot

	return ctx, span, nil
}

// 结束Span
func (tc *TracerSpan) EndSpan(ctx context.Context, span trace.Span, err error) {
	if span == nil {
		return
	}

	traceStatusCode := codes.Ok
	traceStatusDesc := codes.Ok.String()
	traceLevel := "INFO"

	if err != nil {
		span.RecordError(err)
		traceStatusCode = codes.Error
		traceStatusDesc = err.Error()
		traceLevel = "ERROR"
	}

	// 设置属性和状态
	span.SetAttributes(
		attribute.String("trace.level", traceLevel),
		attribute.Int64("rpc.status_code", int64(traceStatusCode)),
	)
	span.SetStatus(traceStatusCode, traceStatusDesc)
	span.End()
}

func (tc *TracerSpan) GetTraceID(ctx context.Context) string {
	if span := trace.SpanContextFromContext(ctx); span.HasTraceID() {
		return span.TraceID().String()
	}
	return ""
}

func (tc *TracerSpan) GetSpanID(ctx context.Context) string {
	if span := trace.SpanContextFromContext(ctx); span.HasSpanID() {
		return span.SpanID().String()
	}
	return ""
}

// 获取客户端传递到GRPC的上下文
func (tc *TracerSpan) GetCtxToGrpc(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{}
	tc.propagator.Inject(ctx, carrier)
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.New(nil)
	}
	for key, value := range carrier {
		md.Set(key, value)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// 获取GRPC的上下文
func (tc *TracerSpan) GetCtxForClient(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	carrier := MDAdapter{MD: md}
	if carrier.MD == nil {
		carrier.MD = metadata.New(nil)
	}
	return tc.propagator.Extract(ctx, carrier)
}

// MDAdapter 适配器使 metadata.MD 兼容 TextMapCarrier 接口。
type MDAdapter struct {
	MD metadata.MD
}

func (m MDAdapter) Get(key string) string {
	values := m.MD.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (m MDAdapter) Set(key string, value string) {
	m.MD.Set(key, value)
}

// Keys 方法返回 carrier 中的所有键。
func (m MDAdapter) Keys() []string {
	keys := make([]string, 0, len(m.MD))
	for k := range m.MD {
		keys = append(keys, k)
	}
	return keys
}
