package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		log.Printf("fastgate stopped with an error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	config := service.DefaultConfig()
	flags := flag.NewFlagSet("fastgate", flag.ContinueOnError)
	flags.StringVar(
		&config.ListenAddress,
		"listen",
		config.ListenAddress,
		"TCP address for the operational health server",
	)
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("parse startup configuration: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("parse startup configuration: unexpected positional arguments")
	}

	application, err := service.New(config)
	if err != nil {
		return fmt.Errorf("validate startup configuration: %w", err)
	}

	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen for FastGate health traffic: %w", err)
	}

	return application.Serve(ctx, listener)
}
