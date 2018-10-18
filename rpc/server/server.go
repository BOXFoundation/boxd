// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/log"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpc")

// Config defines the configurations of rpc server
type Config struct {
	Enabled bool       `mapstructure:"enabled"`
	Address string     `mapstructure:"address"`
	Port    int        `mapstructure:"port"`
	HTTP    HTTPConfig `mapstructure:"http"`
}

// HTTPConfig defines the address/port of rest api over http
type HTTPConfig struct {
	Address string `mapstructure:"address"`
	Port    int    `mapstructure:"port"`
}

// Server defines the rpc server
type Server struct {
	cfg *Config

	ChainReader service.ChainReader
	TxHandler   service.TxHandler

	server   *grpc.Server
	gRPCProc goprocess.Process
	wggRPC   sync.WaitGroup

	httpserver *http.Server
	httpProc   goprocess.Process
	wgHTTP     sync.WaitGroup
}

// Service defines the grpc service func
type Service func(s *Server)

var services = make(map[string]Service)

// GatewayHandler defines the register func of http gateway handler for gRPC service
type GatewayHandler func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

var handlers = make(map[string]GatewayHandler)

// RegisterService registers a new gRPC service
func RegisterService(name string, s Service) {
	services[name] = s
}

// RegisterGatewayHandler registers a new http gateway handler for gRPC service
func RegisterGatewayHandler(name string, h GatewayHandler) {
	handlers[name] = h
}

// RegisterServiceWithGatewayHandler registers a gRPC service with gateway handler
func RegisterServiceWithGatewayHandler(name string, s Service, h GatewayHandler) {
	services[name] = s
	handlers[name] = h
}

// GRPCServer interface breaks cycle import dependency
type GRPCServer interface {
	GetChainReader() service.ChainReader
	GetTxHandler() service.TxHandler
	Stop()
}

// NewServer creates a RPC server instance.
func NewServer(parent goprocess.Process, cfg *Config, cr service.ChainReader, txh service.TxHandler) (*Server, error) {
	var server = &Server{
		cfg:         cfg,
		ChainReader: cr,
		TxHandler:   txh,
	}

	server.gRPCProc = parent.Go(server.servegRPC)

	return server, nil
}

// GetChainReader returns an interface to observe chain state
func (s *Server) GetChainReader() service.ChainReader {
	return s.ChainReader
}

// GetTxHandler returns a handler to deal with transactions
func (s *Server) GetTxHandler() service.TxHandler {
	return s.TxHandler
}

func (s *Server) servegRPC(proc goprocess.Process) {
	var addr = fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	logger.Infof("Starting RPC:gRPC server at %s", addr)
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	s.server = grpc.NewServer()

	// regist all gRPC services for the server
	for name, service := range services {
		logger.Debugf("register gRPC service: %s", name)
		service(s)
	}

	go func() {
		s.wggRPC.Add(1)
		defer s.wggRPC.Done()

		if err := s.server.Serve(lis); err != nil {
			logger.Errorf("failed to serve gRPC: %v", err)
			go proc.Close()
		}
	}()

	// start gRPC gateway
	s.httpProc = proc.Go(s.serveHTTP)

	select {
	case <-proc.Closing():
		logger.Info("Shutting down RPC:gRPC server...")
		s.server.GracefulStop()
		lis.Close()
	}

	s.wggRPC.Wait()
	logger.Info("RPC:gRPC server is down.")
}

func (s *Server) serveHTTP(proc goprocess.Process) {
	var addr = fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)

	// register http gateway handlers
	// mux := runtime.NewServeMux()
	// see https://github.com/grpc-ecosystem/grpc-gateway/issues/233
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(
		runtime.MIMEWildcard,
		&runtime.JSONPb{
			OrigName:     true,
			EmitDefaults: true,
		},
	))
	opts := []grpc.DialOption{grpc.WithInsecure()}
	for name, handler := range handlers {
		logger.Debugf("register gRPC gateway handler: %s", name)
		if err := handler(goprocessctx.OnClosingContext(proc), mux, addr, opts); err != nil {
			logger.Fatalf("failed register gRPC http gateway handler: %s", name)
		}
	}

	var httpendpoint = fmt.Sprintf("%s:%d", s.cfg.HTTP.Address, s.cfg.HTTP.Port)
	s.httpserver = &http.Server{Addr: httpendpoint, Handler: mux}
	go func() {
		s.wgHTTP.Add(1)
		defer s.wgHTTP.Done()

		logger.Infof("Starting RPC:http server at %s", httpendpoint)
		if err := s.httpserver.ListenAndServe(); err != http.ErrServerClosed {
			// close proc only if the err is not ErrServerClosed
			logger.Errorf("gRPC http gateway error: %v", err)
			go proc.Close()
		}
	}()

	select {
	case <-proc.Closing():
		logger.Info("Shutting down RPC:http server...")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		s.httpserver.Shutdown(ctx)
	}

	s.wgHTTP.Wait()
	logger.Info("RPC:http server is down.")
}

// Stop the rpc server
func (s *Server) Stop() {
	go s.gRPCProc.Close()
}
