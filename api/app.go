// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TeneficGames/podium/config"
	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/enriching"
	enrichercache "github.com/TeneficGames/podium/leaderboard/enriching/cache"
	lservice "github.com/TeneficGames/podium/leaderboard/service"
	"github.com/TeneficGames/podium/log"
	"github.com/TeneficGames/podium/observability"
	api "github.com/TeneficGames/podium/proto/podium/api/v1"
	uuid "github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rcrowley/go-metrics"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
)

// JSON type
type JSON map[string]interface{}

// App is a struct that represents a podium Application
type App struct {
	api.UnimplementedPodiumServiceServer

	ConfigPath string
	Debug      bool

	// HTTP endpoint for HTTP requests. Built after calling Start. Format: 127.0.0.1:8080
	HTTPEndpoint string

	// GRPC endpoint for GRPC requests. Built after calling Start. Format: 127.0.0.1:8081
	GRPCEndpoint string

	httpReady, grpcReady chan bool

	Config          *viper.Viper
	ParsedConfig    *config.PodiumConfig
	Enricher        enriching.Enricher
	Errors          metrics.EWMA
	grpcServer      *grpc.Server
	httpServer      *http.Server
	ID              uuid.UUID
	Logger          *zap.Logger
	Leaderboards    lservice.Leaderboard
	observability   *observability.Provider
	requestDuration metric.Float64Histogram
}

// New returns a new podium Application.
// If httpPort is sent as zero, a random port will be selected (the same will happen for grpcPort)
func New(host string, httpPort, grpcPort int, configPath string, debug bool, logger *zap.Logger) (*App, error) {
	app := &App{
		HTTPEndpoint: fmt.Sprintf("%s:%d", host, httpPort),
		GRPCEndpoint: fmt.Sprintf("%s:%d", host, grpcPort),
		httpReady:    make(chan bool, 1),
		grpcReady:    make(chan bool, 1),
		ConfigPath:   configPath,
		Config:       viper.New(),
		Debug:        debug,
		Logger:       logger,
		ID:           uuid.New(),
	}
	err := app.configure()
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (app *App) getStatusCodeFromError(err error) (*status.Status, int) {
	var statusCode int
	st, ok := status.FromError(err)
	if !ok {
		statusCode = http.StatusInternalServerError
	} else {
		statusCode = runtime.HTTPStatusFromCode(st.Code())
	}
	return st, statusCode
}

// Configure instantiates the required dependencies for podium Application
func (app *App) configure() error {
	app.setConfigurationDefaults()

	err := app.loadConfiguration()
	if err != nil {
		return err
	}

	err = app.configureObservability()
	if err != nil {
		return err
	}

	err = app.configureEnrichment()
	if err != nil {
		app.shutdownObservability()
		return err
	}

	err = app.configureApplication()
	if err != nil {
		app.shutdownObservability()
		return err
	}

	return nil
}

func (app *App) configureObservability() error {
	provider, err := observability.New(context.Background(), "podium")
	if err != nil {
		return err
	}
	app.observability = provider
	app.requestDuration, err = otel.Meter("github.com/TeneficGames/podium/api").Float64Histogram(
		"podium.rpc.server.duration",
		metric.WithDescription("Duration of gRPC server requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		app.shutdownObservability()
		return fmt.Errorf("create request duration metric: %w", err)
	}
	return nil
}

func (app *App) setConfigurationDefaults() {
	app.Config.SetDefault("healthcheck.workingText", "WORKING")
	app.Config.SetDefault("graceperiod.ms", 50)
	app.Config.SetDefault("api.maxReturnedMembers", 2000)
	app.Config.SetDefault("api.maxReadBufferSize", 32000)
	app.Config.SetDefault("redis.host", "localhost")
	app.Config.SetDefault("redis.port", 6379)
	app.Config.SetDefault("redis.password", "")
	app.Config.SetDefault("redis.db", 0)
	app.Config.SetDefault("redis.connectionTimeout", 200)
	app.Config.SetDefault("redis.cluster.enabled", false)
}

func (app *App) loadConfiguration() error {
	app.Config.SetConfigFile(app.ConfigPath)
	app.Config.SetEnvPrefix("podium")
	app.Config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	app.Config.AutomaticEnv()

	if err := app.Config.ReadInConfig(); err == nil {
		app.Logger.Info("Loaded config file.", zap.String("configFile", app.Config.ConfigFileUsed()))
	} else {
		return fmt.Errorf("could not load configuration file from: %s", app.ConfigPath)
	}

	app.ParsedConfig = &config.PodiumConfig{}
	if err := app.Config.Unmarshal(app.ParsedConfig, config.DecodeHook()); err != nil {
		return fmt.Errorf("could not parse configuration file: %w", err)
	}

	return nil
}

func (app *App) configureEnrichment() error {
	providers := make(map[string]enriching.Provider, len(app.ParsedConfig.Enrichment.Providers))
	for tenantID, provider := range app.ParsedConfig.Enrichment.Providers {
		enrichmentProvider := enriching.Provider{
			Endpoint: provider.Endpoint,
			Headers:  provider.Headers,
			Mode:     enriching.FailureMode(provider.Mode),
			Retry: enriching.RetryConfig{
				MaxAttempts:    provider.Retry.MaxAttempts,
				InitialBackoff: provider.Retry.InitialBackoff,
				MaxBackoff:     provider.Retry.MaxBackoff,
			},
		}
		if err := enrichmentProvider.Validate(); err != nil {
			return fmt.Errorf("invalid enrichment provider for tenant %q: %w", tenantID, err)
		}
		providers[tenantID] = enrichmentProvider
	}

	enricher := enriching.NewEnricher(
		enriching.WithLogger(app.Logger),
		enriching.WithProviders(providers),
		enriching.WithRequestTimeout(app.ParsedConfig.Enrichment.RequestTimeout),
	)
	instrumentedEnricher, err := enriching.NewInstrumentedEnricher(enricher)
	if err != nil {
		return fmt.Errorf("configure enrichment instrumentation: %w", err)
	}
	app.Enricher = instrumentedEnricher

	if app.ParsedConfig.Enrichment.Cache.Addr != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     app.ParsedConfig.Enrichment.Cache.Addr,
			Password: app.ParsedConfig.Enrichment.Cache.Password,
		})

		enrichCache := enrichercache.NewEnricherRedisCache(redisClient)
		instrumentedCache, err := enrichercache.NewInstrumentedCache(enrichCache)
		if err != nil {
			return fmt.Errorf("configure enrichment cache instrumentation: %w", err)
		}
		app.Enricher = enrichercache.NewCachedEnricher(
			instrumentedCache,
			app.Enricher,
			enrichercache.WithLogger(app.Logger),
			enrichercache.WithTTL(app.ParsedConfig.Enrichment.Cache.TTL),
		)

		app.Logger.Info("Enrichment cache configured successfully.")
	}

	app.Logger.Info("Enrichment configured successfully.")
	return nil
}

// OnErrorHandler handles panics
func (app *App) OnErrorHandler(err error, stack []byte) {
	app.onErrorHandler(context.Background(), err, stack)
}

func (app *App) onErrorHandler(ctx context.Context, err error, stack []byte) {
	app.Logger.Error(
		"Panic occurred.",
		zap.Any("panicText", err),
		zap.String("stack", string(stack)),
	)

	observability.CaptureException(
		ctx,
		err,
		map[string]string{"source": "app", "type": "panic"},
		map[string]interface{}{"stack": string(stack)},
	)
}

func (app *App) configureApplication() error {
	app.Errors = metrics.NewEWMA15()

	go func() {
		app.Errors.Tick()
		time.Sleep(5 * time.Second)
	}()

	client, err := app.createAndConfigureLeaderboardClient()
	if err != nil {
		return err
	}
	app.Leaderboards = client

	return nil
}

func (app *App) createAndConfigureLeaderboardClient() (lservice.Leaderboard, error) {
	client := app.createLeaderboardClient()
	err := client.Healthcheck(context.Background())

	if err != nil {
		return nil, err
	}

	app.Logger.Info("Leaderboard client creation successful.")
	return client, nil
}

func (app *App) createLeaderboardClient() lservice.Leaderboard {
	shouldRunOnCluster := app.Config.GetBool("redis.cluster.enabled")
	addrs := app.Config.GetStringSlice("redis.addrs")
	password := app.Config.GetString("redis.password")
	host := app.Config.GetString("redis.host")
	port := app.Config.GetInt("redis.port")
	db := app.Config.GetInt("redis.db")

	logger := app.Logger.With(
		zap.String("operation", "createLeaderboardClient"),
		zap.Strings("addrs", addrs),
		zap.Bool("cluster", shouldRunOnCluster),
		zap.String("url", fmt.Sprintf("redis://:<REDACTED>@%s:%v/%v", host, port, db)),
	)

	leaderboardService := lservice.NewService(database.NewRedisDatabase(database.RedisOptions{
		ClusterEnabled: shouldRunOnCluster,
		Addrs:          addrs,
		Host:           host,
		Password:       password,
		Port:           port,
		DB:             db,
	}))

	logger.Info("Creating leaderboard client.")

	return leaderboardService
}

// AddError rate statistics
func (app *App) AddError() {
	app.Errors.Update(1)
}

// Start starts listening for web requests at specified host and port
func (app *App) Start(ctx context.Context) error {
	defer app.shutdownObservability()

	l := app.Logger.With(
		zap.String("source", "app"),
		zap.String("operation", "Start"),
		zap.String("HTTPEndpoint", app.HTTPEndpoint),
		zap.String("GRPCEndPoint", app.GRPCEndpoint),
	)

	var listenConfig net.ListenConfig
	grpcLis, err := listenConfig.Listen(ctx, "tcp", app.GRPCEndpoint)
	if err != nil {
		return fmt.Errorf("error trying to listen for connections: %w", err)
	}
	app.GRPCEndpoint = grpcLis.Addr().String()

	httpLis, err := listenConfig.Listen(ctx, "tcp", app.HTTPEndpoint)
	if err != nil {
		return fmt.Errorf("error listening on HTTPEndpoint: %w", err)
	}
	app.HTTPEndpoint = httpLis.Addr().String()

	//errch is the channel for retrieving errors from server goroutines.
	errch := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := app.startGRPCServer(grpcLis); err != nil {
			errch <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := app.startHTTPServer(ctx, httpLis); err != nil {
			errch <- err
		}
	}()

	stopped := make(chan bool, 1)
	go func() {
		wg.Wait()
		stopped <- true
	}()

	log.I(l, "app started")
	sg := make(chan os.Signal, 1)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	defer signal.Stop(sg)

	// stop server
	select {
	case s := <-sg:
		graceperiod := app.Config.GetInt("graceperiod.ms")
		log.I(l, "shutting down", func(cm log.CM) {
			cm.Write(zap.String("signal", fmt.Sprintf("%v", s)),
				zap.Int("graceperiod", graceperiod))
		})
		app.GracefullStop()
		time.Sleep(time.Duration(graceperiod) * time.Millisecond)
	case err := <-errch:
		app.Logger.Error("Err on start server", zap.Error(err))
		return err
	case <-ctx.Done():
		app.GracefullStop()
		return ctx.Err()
	case <-stopped:
	}
	log.I(l, "app stopped")
	return nil
}

func (app *App) startGRPCServer(lis net.Listener) error {
	var basicAuthInterceptor grpc.UnaryServerInterceptor

	basicAuthUser := app.Config.GetString("basicauth.username")
	if basicAuthUser == "" {
		basicAuthInterceptor = app.noAuthMiddleware
	} else {
		basicAuthInterceptor = grpcauth.UnaryServerInterceptor(app.basicAuthMiddleware)
	}

	app.grpcServer = grpc.NewServer(grpc.ChainUnaryInterceptor(
		basicAuthInterceptor,
		app.loggerMiddleware,
		app.recoveryMiddleware,
		app.responseTimeMetricsMiddleware,
	), grpc.StatsHandler(otelgrpc.NewServerHandler()))
	api.RegisterPodiumServiceServer(app.grpcServer, app)

	app.grpcReady <- true
	if err := app.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("error trying to serve with grpc server: %w", err)
	}

	return nil
}

func (app *App) applicationErrorHandler(_ context.Context, _ *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, rpcErr error) {

	w.Header().Set("Content-Type", "application/json")
	st, s := app.getStatusCodeFromError(rpcErr)

	w.WriteHeader(s)

	type errorBody struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
	}

	body := &errorBody{
		Success: false,
		Reason:  st.Message(),
	}

	buf, err := marshaler.Marshal(body)
	if err != nil {
		app.Logger.Error("Failed to marshal error body,", zap.Error(err))
		return
	}
	if _, err := w.Write(buf); err != nil {
		app.Logger.Error("Failed to write response.,", zap.Error(err))
	}
}

func (app *App) startHTTPServer(ctx context.Context, lis net.Listener) error {
	gatewayMux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{MarshalOptions: protojson.MarshalOptions{EmitUnpopulated: true}}),
		runtime.WithErrorHandler(app.applicationErrorHandler),
		runtime.WithIncomingHeaderMatcher(customHeadersMatcher),
	)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	if err := api.RegisterPodiumServiceHandlerFromEndpoint(ctx, gatewayMux, app.GRPCEndpoint, opts); err != nil {
		return fmt.Errorf("error registering multiplexer for grpc gateway: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", removeTrailingSlashMiddleware{addVersionMiddleware{gatewayMux}})
	mux.HandleFunc("/healthcheck", addVersionHandlerFunc(app.healthCheckHandler))
	mux.HandleFunc("/status", addVersionHandlerFunc(app.statusHandler))
	app.httpServer = &http.Server{
		Addr:    app.HTTPEndpoint,
		Handler: otelhttp.NewHandler(mux, "podium.http"),
	}

	app.httpReady <- true
	if err := app.httpServer.Serve(lis); err != http.ErrServerClosed {
		return fmt.Errorf("error listening and serving http requests: %w", err)
	}

	return nil
}

// WaitForReady blocks until App is ready to serve requests or the timeout is reached.
// An error is returned on timeout.
func (app *App) WaitForReady(d time.Duration) error {
	isReady := func(c chan bool) bool {
		select {
		case <-c:
			return true
		case <-time.After(d):
			return false
		}
	}

	if isReady(app.grpcReady) && isReady(app.httpReady) {
		return nil
	}
	return fmt.Errorf("timed out waiting for endpoints")
}

// GracefullStop attempts to stop the server.
func (app *App) GracefullStop() {
	if app.grpcServer != nil {
		app.grpcServer.GracefulStop()
	}
	if app.httpServer != nil {
		if err := app.httpServer.Shutdown(context.Background()); err != nil {
			app.Logger.Error("HTTP server Shutdown.", zap.Error(err))
		}
	}
	app.shutdownObservability()
}

func (app *App) shutdownObservability() {
	if app.observability == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.observability.Shutdown(ctx); err != nil {
		app.Logger.Error("Observability shutdown.", zap.Error(err))
	}
}

func customHeadersMatcher(key string) (string, bool) {
	if strings.EqualFold(key, TenantIDHeaderKey) {
		return key, true
	}
	return runtime.DefaultHeaderMatcher(key)
}
