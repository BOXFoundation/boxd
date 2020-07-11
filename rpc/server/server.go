// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/util"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	"github.com/rs/cors"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var logger = log.NewLogger("rpc")

// Define const
const (
	DefaultGrpcLimits = 128
	DefaultHTTPLimits = 128
)

//
var (
	ErrAPINotSupported = errors.New("api not supported")
	ErrIPNotAllowed    = errors.New("allowed only users in white list")
)

// Config defines the configurations of rpc server
type Config struct {
	Enable          bool         `mapstructure:"enable"`
	Address         string       `mapstructure:"address"`
	Port            int          `mapstructure:"port"`
	HTTP            HTTPConfig   `mapstructure:"http"`
	GrpcLimits      int          `mapstructure:"grpc_limits"`
	HTTPLimits      int          `mapstructure:"http_limits"`
	HTTPCors        []string     `mapstructure:"http_cors"`
	Faucet          FaucetConfig `mapstructure:"faucet"`
	SubScribeBlocks bool         `mapstructure:"subscribe_blocks"`
	SubScribeLogs   bool         `mapstructure:"subscribe_logs"`
	AdminIPs        []string     `mapstructure:"admin_ips"`
}

//FaucetConfig  defines the faucet config
type FaucetConfig struct {
	Keyfile      string   `mapstructure:"keyfile"`
	WhiteList    []string `mapstructure:"white_list"`
	AmountPerSec uint64   `mapstructure:"amount_per_sec"`
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
	TableReader service.TableReader
	WalletAgent service.WalletAgent
	eventBus    eventbus.Bus
	server      *grpc.Server
	gRPCProc    goprocess.Process
	wggRPC      sync.WaitGroup

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
	GetWalletAgent() service.WalletAgent
	GetEventBus() eventbus.Bus
	Proc() goprocess.Process
	Stop()
}

// NewServer creates a RPC server instance.
func NewServer(
	parent goprocess.Process, cfg *Config, cr service.ChainReader,
	txh service.TxHandler, wa service.WalletAgent, peer *p2p.BoxPeer,
	bus eventbus.Bus,
) *Server {
	var server = &Server{
		cfg:         cfg,
		ChainReader: cr,
		TxHandler:   txh,
		TableReader: peer,
		eventBus:    bus,
		WalletAgent: wa,
		gRPCProc:    goprocess.WithParent(parent),
	}

	return server
}

// implement interface service.Server
var _ service.Server = (*Server)(nil)

// Run gRPC service
func (s *Server) Run() error {
	s.gRPCProc.Go(s.servegRPC)
	return nil
}

// Proc returns the goprocess
func (s *Server) Proc() goprocess.Process {
	return s.gRPCProc
}

// Stop gRPC service
func (s *Server) Stop() {
	s.gRPCProc.Close()
}

// GetChainReader returns an interface to observe chain state
func (s *Server) GetChainReader() service.ChainReader {
	return s.ChainReader
}

// GetTxHandler returns a handler to deal with transactions
func (s *Server) GetTxHandler() service.TxHandler {
	return s.TxHandler
}

// GetTableReader returns an interface to query routing table
func (s *Server) GetTableReader() service.TableReader {
	return s.TableReader
}

// GetWalletAgent returns the wallet related service handler
func (s *Server) GetWalletAgent() service.WalletAgent {
	return s.WalletAgent
}

// GetEventBus returns a interface to publish events
func (s *Server) GetEventBus() eventbus.Bus {
	return s.eventBus
}

func (s *Server) servegRPC(proc goprocess.Process) {
	var addr = fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	logger.Infof("Starting RPC:gRPC server at %s", addr)
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	var interceptor grpc.UnaryServerInterceptor
	interceptor = func(
		ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		//start := time.Now()
		//uid := uuid.NewV4()
		resp, err := handler(ctx, req)
		//logger.Debugf("grpc access log: %v %v %v\n, resp: %v, err: %v", uid, info.FullMethod, time.Since(start), resp, err)
		return resp, err
	}
	opts = append(opts, grpc.UnaryInterceptor(interceptor))

	s.server = grpc.NewServer(opts...)

	// regist all gRPC services for the server
	s.registerSerivices()
	for name, service := range services {
		logger.Debugf("register gRPC service: %s", name)
		service(s)
	}

	// Limit the total number of grpc connections.
	grpcLimits := s.cfg.GrpcLimits
	if grpcLimits == 0 {
		grpcLimits = DefaultGrpcLimits
	}

	lis = netutil.LimitListener(lis, grpcLimits)

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
	s.httpserver = &http.Server{Addr: httpendpoint, Handler: s.withHTTPLimits(mux)}
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

func (s *Server) withHTTPLimits(h http.Handler) http.Handler {
	httpLimit := s.cfg.HTTPLimits
	if httpLimit == 0 {
		httpLimit = DefaultHTTPLimits
	}
	httpCh := make(chan bool, httpLimit)

	c := cors.New(cors.Options{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "DELETE"},
		AllowedOrigins: s.cfg.HTTPCors,
		MaxAge:         600,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case httpCh <- true:
			defer func() { <-httpCh }()
			c.Handler(h).ServeHTTP(w, r)
		default:
			serviceUnavailableHandler(w, r)
		}
	})
}

func serviceUnavailableHandler(w http.ResponseWriter, r *http.Request) {
	logger.Errorf("Sorry, the server is busy due to too many requests")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("{\"Err:\",\"Sorry, the server is busy due to too many requests.\nPlease try again later.\"}"))
}

func (s *Server) registerSerivices() {
	if len(s.cfg.AdminIPs) > 0 {
		RegisterServiceWithGatewayHandler(
			"admincontrol",
			func(s *Server) {
				rpcpb.RegisterAdminControlServer(
					s.server,
					&adminControl{
						server:      s,
						whiteList:   s.cfg.AdminIPs,
						TableReader: s.GetTableReader(),
					},
				)
			},
			rpcpb.RegisterAdminControlHandlerFromEndpoint,
		)
	}
	if !s.cfg.Enable {
		return
	}
	RegisterServiceWithGatewayHandler(
		"control",
		func(s *Server) {
			rpcpb.RegisterContorlCommandServer(s.server, &ctlserver{server: s})
		},
		rpcpb.RegisterContorlCommandHandlerFromEndpoint,
	)
	RegisterServiceWithGatewayHandler(
		"database",
		func(s *Server) {
			rpcpb.RegisterDatabaseCommandServer(s.server, &dbserver{server: s})
		},
		rpcpb.RegisterDatabaseCommandHandlerFromEndpoint,
	)
	RegisterServiceWithGatewayHandler(
		"tx",
		func(s *Server) {
			rpcpb.RegisterTransactionCommandServer(s.server, &txServer{server: s})
		},
		rpcpb.RegisterTransactionCommandHandlerFromEndpoint,
	)
	RegisterServiceWithGatewayHandler(
		"webapi",
		func(s *Server) {
			was := newWebAPIServer(s)
			rpcpb.RegisterWebApiServer(s.server, was)
		},
		rpcpb.RegisterWebApiHandlerFromEndpoint,
	)
	RegisterServiceWithGatewayHandler(
		"faucet",
		registerFaucet,
		rpcpb.RegisterFaucetHandlerFromEndpoint,
	)
}

func isInIPs(ctx context.Context, ips []string) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	cliIPs := md["x-forwarded-for"]
	if util.InStrings("*", ips) {
		return true
	}
	for _, ip := range cliIPs {
		if util.InStrings(ip, ips) {
			return true
		}
	}
	return false
}
