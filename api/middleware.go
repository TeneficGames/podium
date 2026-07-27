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
	"encoding/base64"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/TeneficGames/podium/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
)

func (app *App) noAuthMiddleware(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return handler(ctx, req)
}

func (app *App) basicAuthMiddleware(ctx context.Context) (context.Context, error) {
	token, err := grpc_auth.AuthFromMD(ctx, "basic")
	if err != nil {
		return nil, err
	}

	auth := app.Config.GetString("basicauth.username") + ":" + app.Config.GetString("basicauth.password")

	if token != base64.StdEncoding.EncodeToString([]byte(auth)) {
		return nil, status.Error(codes.Unauthenticated, "invalid auth token")
	}
	return ctx, nil
}

func (app *App) loggerMiddleware(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	l := app.Logger.With(
		zap.String("source", "request"),
	)

	// all except latency to string
	var statusCode int
	var latency time.Duration
	var startTime, endTime time.Time

	startTime = time.Now()

	h, err := handler(ctx, req)

	// no time.Since in order to format it well after
	endTime = time.Now()
	latency = endTime.Sub(startTime)

	_, statusCode = app.getStatusCodeFromError(err)

	method := info.FullMethod
	reqLog := l.With(
		zap.String("method", method),
		zap.Time("endTime", endTime),
		zap.Int("statusCode", statusCode),
		zap.Duration("latency", latency),
	)

	// request failed
	if statusCode > 399 && statusCode < 500 {
		log.D(reqLog, "Request failed.")
		return h, err
	}

	// request is ok, but server failed
	if statusCode > 499 {
		log.D(reqLog, "Response failed.")
		return h, err
	}

	// Everything went ok
	log.D(reqLog, "Request successful.")
	return h, err
}

// Serve executes on error handler when errors happen
func (app *App) recoveryMiddleware(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response interface{}, responseErr error) {
	defer func() {
		if err := recover(); err != nil {
			eError, ok := err.(error)
			if !ok {
				eError = fmt.Errorf("%v", err)
			}
			app.onErrorHandler(ctx, eError, debug.Stack())
			response = nil
			responseErr = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}

func (app *App) responseTimeMetricsMiddleware(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	startTime := time.Now()
	h, err := handler(ctx, req)
	timeUsed := time.Since(startTime)
	method := info.FullMethod

	app.requestDuration.Record(ctx, timeUsed.Seconds(),
		metric.WithAttributes(
			attribute.String("rpc.method", method),
			attribute.Int("rpc.grpc.status_code", int(status.Code(err))),
		),
	)

	return h, err
}

type addVersionMiddleware struct {
	Handler http.Handler
}

func addVersionHeaders(w http.ResponseWriter) {
	w.Header().Set("Server", fmt.Sprintf("Podium/v%s", VERSION))
	w.Header().Set("Podium-Server", fmt.Sprintf("Podium/v%s", VERSION))
}

func (m addVersionMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addVersionHeaders(w)
	m.Handler.ServeHTTP(w, r)
}

func addVersionHandlerFunc(f func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		addVersionHeaders(w)
		f(w, r)
	}
}

type removeTrailingSlashMiddleware struct {
	Handler http.Handler
}

func (m *removeTrailingSlashMiddleware) removeTrailingSlash(path string) string {
	l := len(path) - 1
	if l > 0 && path != "/" && path[l] == '/' {
		return path[:l]
	}
	return path
}

func (m removeTrailingSlashMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = m.removeTrailingSlash(r.URL.Path)
	m.Handler.ServeHTTP(w, r)
}
