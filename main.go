package main

import (
	"context"
	"errors"
	traces2 "github.com/huangchunlong818/go-trace/trace"
)

func main() {
	ctx := context.Background()
	traces, err := traces2.GetNewTracerSpan(traces2.TraceConfig{
		Version:          "v1.0",
		Endpoint:         "test",
		Project:          "test",
		Instance:         "test",
		AccessKeyId:      "test",
		AccessKeySecret:  "test",
		ServiceName:      "aaaaaaaaaa",
		ServiceNamespace: "bbbbbbbbbbbbbbbb",
	})
	if err != nil {
		panic(err)
	}

	//这里调用具体逻辑
	err = errors.New("test error")

	//开启span
	ctx, span, err := traces.StartSpan(ctx, "testTraceSpan", "client")
	if err != nil {
		panic(err)
	}

	traces.EndSpan(ctx, span, err)

	select {}
}
