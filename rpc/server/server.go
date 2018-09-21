// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"sync"

	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/jbenet/goprocess"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpc")

// Config defines the configurations of rpc server
type Config struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`
	Port    int    `mapstructure:"port"`
}

// Server defines the rpc server
type Server struct {
	cfg    *Config
	server *grpc.Server
	proc   goprocess.Process

	wg sync.WaitGroup
}

// Service defines the grpc service func
type Service func(s *grpc.Server)

var services = make(map[string]Service)

// RegisterService registers a new grpc service
func RegisterService(name string, s Service) {
	services[name] = s
}

// NewServer creates a RPC server instance.
func NewServer(parent goprocess.Process, cfg *Config) (*Server, error) {
	var server = &Server{
		cfg: cfg,
	}

	server.proc = parent.Go(server.serve)

	return server, nil
}

func (s *Server) serve(proc goprocess.Process) {
	var addr = fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	logger.Infof("Starting gRPC server at %s", addr)
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	s.server = grpc.NewServer()

	// regist all grpc services for the server
	for name, service := range services {
		logger.Debugf("register grpc service: %s", name)
		service(s.server)
	}

	go func() {
		s.wg.Add(1)
		defer s.wg.Done()

		if err := s.server.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	select {
	case <-proc.Closing():
		logger.Info("Shutting down rpc server...")
		s.server.GracefulStop()
	}

	s.wg.Wait()
	logger.Info("RPC server is down.")
}

// Stop the rpc server
func (s *Server) Stop() {
	s.proc.Close()
}
