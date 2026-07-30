module github.com/TeneficGames/podium

go 1.26.0

toolchain go1.26.5

require (
	github.com/TeneficGames/podium/leaderboard v1.0.0
	github.com/TeneficGames/podium/proto v1.0.0
	github.com/getsentry/sentry-go v0.48.0
	github.com/getsentry/sentry-go/otel v0.48.0
	github.com/google/uuid v1.6.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0
	github.com/mailru/easyjson v0.9.2
	github.com/mitchellh/mapstructure v1.5.0
	github.com/onsi/ginkgo/v2 v2.32.0
	github.com/onsi/gomega v1.42.1
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9
	github.com/redis/go-redis/v9 v9.21.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
	github.com/valyala/fasthttp v1.72.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.44.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0
	go.opentelemetry.io/otel/metric v1.44.0
	go.opentelemetry.io/otel/sdk v1.44.0
	go.opentelemetry.io/otel/sdk/metric v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
	go.uber.org/mock v0.6.0
	go.uber.org/zap v1.28.0
	golang.org/x/sync v0.22.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/alicebob/miniredis/v2 v2.38.0 // indirect
	github.com/andybalholm/brotli v1.2.2 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260709232956-b9395ee17fa0 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260724162435-b2f20204f0df // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260724162435-b2f20204f0df // indirect
)

replace github.com/TeneficGames/podium/leaderboard v1.0.0 => ./leaderboard

replace github.com/TeneficGames/podium/proto v1.0.0 => ./proto
