package args

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/worldline-go/chore/internal/config"
	"github.com/worldline-go/chore/internal/server"
	"github.com/worldline-go/chore/internal/store"
	"github.com/worldline-go/initializer"
	"github.com/worldline-go/tell"

	// Add flow nodes to register in control flow algorithm.
	_ "github.com/worldline-go/chore/pkg/flow/nodes"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/worldline-go/igconfig"
	"github.com/worldline-go/igconfig/loader"
	"github.com/worldline-go/logz"
)

type overrideHold struct {
	Memory *string
	Value  string
}

type ctxValue string

const ctxKeyWg ctxValue = "wg"

var rootCmd = &cobra.Command{
	Use:     "chore",
	Short:   "control flow runner",
	Long:    config.Banner("request with templates"),
	Version: config.AppVersion,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		if err := logz.SetLogLevel(config.Application.LogLevel); err != nil {
			return err //nolint:wrapcheck // no need
		}

		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// load configuration
		if err := loadConfig(cmd.Context(), cmd.Flags().Visit); err != nil {
			return err
		}

		if err := runRoot(cmd.Context()); err != nil {
			return err
		}

		return nil
	},
}

func Execute(ctx context.Context, wg *sync.WaitGroup) error {
	ctx = context.WithValue(ctx, ctxKeyWg, wg)

	return rootCmd.ExecuteContext(ctx) //nolint:wrapcheck // no need
}

//nolint:gochecknoinits // cobra init
func init() {
	rootCmd.Flags().StringVarP(&config.Application.Host, "host", "H", config.Application.Host, "Host to listen on")
	rootCmd.Flags().StringVarP(&config.Application.Port, "port", "P", config.Application.Port, "Port to listen on")
	rootCmd.PersistentFlags().StringVarP(&config.Application.LogLevel, "log-level", "l", config.Application.LogLevel, "Log level")
}

// override function hold first values of definitions.
// Use with pflag visit function.
func override(ow map[string]overrideHold) {
	ow["host"] = overrideHold{&config.Application.Host, config.Application.Host}
	ow["port"] = overrideHold{&config.Application.Port, config.Application.Port}
	ow["log-level"] = overrideHold{&config.Application.LogLevel, config.Application.LogLevel}
}

func loadConfig(ctx context.Context, visit func(fn func(*pflag.Flag))) error {
	overrideValues := make(map[string]overrideHold)
	override(overrideValues)

	logConfig := log.With().Str("component", "config").Logger()
	ctxConfig := logConfig.WithContext(ctx)

	loaders := []loader.Loader{
		&loader.Consul{},
		&loader.Vault{},
		&loader.File{},
		&loader.Env{},
	}

	// stop to lead generic directly
	loader.VaultSecretAdditionalPaths = []loader.AdditionalPath{}

	if err := igconfig.LoadWithLoadersWithContext(ctxConfig, "", &config.LoadConfig, loaders[3]); err != nil {
		return fmt.Errorf("unable to load prefix settings: %w", err)
	}

	loader.ConsulConfigPathPrefix = config.LoadConfig.Prefix.Consul
	loader.VaultSecretBasePath = config.LoadConfig.Prefix.Vault

	if err := igconfig.LoadWithLoadersWithContext(ctxConfig, config.LoadConfig.AppName, &config.Application, loaders...); err != nil {
		return fmt.Errorf("unable to load configuration settings: %w", err)
	}

	// override used cmd values
	visit(func(f *pflag.Flag) {
		if v, ok := overrideValues[f.Name]; ok {
			*v.Memory = v.Value
		}
	})

	// set log again to get changes
	if err := logz.SetLogLevel(config.Application.LogLevel); err != nil {
		return err //nolint:wrapcheck // no need
	}

	// print loaded object
	log.Info().Object("config", igconfig.Printer{Value: config.Application}).Msg("loaded config")

	return nil
}

func runRoot(ctx context.Context) error {
	wg, _ := ctx.Value(ctxKeyWg).(*sync.WaitGroup)
	if wg == nil {
		return fmt.Errorf("wg not found in context")
	}

	// appname and version
	// config.Banner("custom send request with templates"),
	log.WithLevel(zerolog.NoLevel).Msgf(
		"%s [%s] buildCommit=[%s] buildDate=[%s] %s",
		strings.ToTitle(config.AppName), config.AppVersion,
		config.AppBuildCommit, config.AppBuildDate, config.Application.Env,
	)

	// telemetry initialize
	collector, err := tell.New(ctx, config.Application.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to init telemetry; %w", err)
	}
	defer collector.Shutdown() //nolint:errcheck // no need

	defer func() {
		log.Info().Msg("application shutdown")
	}()

	// open db connection
	dbConn, err := store.OpenConnection(config.Application.Store.Type, map[string]interface{}{
		"host":     config.Application.Store.Host,
		"port":     config.Application.Store.Port,
		"password": config.Application.Store.Password,
		"user":     config.Application.Store.User,
		"dbName":   config.Application.Store.DBName,
		"schema":   config.Application.Store.Schema,
		"timeZone": config.Application.Store.TimeZone,
		"dsn":      config.Application.Store.DBDataSource,
	})
	if err != nil {
		return fmt.Errorf("cannot open db: %w", err)
	}

	// get generic interface and close in defer
	dbGeneric, err := dbConn.DB()
	if err != nil {
		return fmt.Errorf("cannot get generic interface of gorm: %w", err)
	}

	// set generic db settings
	if config.Application.Store.MaxIdleConns > config.Application.Store.MaxOpenConns {
		config.Application.Store.MaxIdleConns = config.Application.Store.MaxOpenConns
	}

	dbGeneric.SetConnMaxIdleTime(config.Application.Store.ConnMaxIdleTime)
	dbGeneric.SetConnMaxLifetime(config.Application.Store.ConnMaxLifeTime)
	dbGeneric.SetMaxIdleConns(config.Application.Store.MaxIdleConns)
	dbGeneric.SetMaxOpenConns(config.Application.Store.MaxOpenConns)

	defer dbGeneric.Close()

	// migrate database
	if err := store.AutoMigrate(ctx, dbConn); err != nil {
		return fmt.Errorf("auto migration: %w", err)
	}

	// server wait
	e, err := server.Set(ctx, wg, dbConn)
	if err != nil {
		return err //nolint:wrapcheck // no need
	}

	initializer.Shutdown.Add(func() error {
		return server.Stop(e)
	}, initializer.WithShutdownName("server"))

	// start server
	if err := server.Start(e); err != nil {
		return err //nolint:wrapcheck // no need
	}

	return nil
}
