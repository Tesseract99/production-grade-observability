package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func initTracer() func(context.Context) error {

	ctx := context.Background()


	// 1. Create the Exporter (Writing to stdout for now)
	// exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Identify your resource (Your Service Name)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("movie-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Create the TraceProvider (The Engine)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// 4. Set the global TraceProvider so the API knows what to use
	otel.SetTracerProvider(tp)

	// Return a cleanup function to flush data before app exit
	return tp.Shutdown
}