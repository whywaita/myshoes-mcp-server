package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/goccy/go-json"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	myshoesapi "github.com/whywaita/myshoes/api/myshoes"
)

var version = "dev-version"
var commit = "dev-commit"
var date = "dev-date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "myshoes MCP Server",
		Long:    `A myshoes MCP server that handles various tools and resources.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		Run: func(_ *cobra.Command, _ []string) {
			logger := initLogger()

			logCommands := viper.GetBool("enable-command-logging")
			cfg := runConfig{
				logger:      logger,
				logCommands: logCommands,
			}
			if err := runStdioServer(cfg); err != nil {
				logger.Error("failed to run stdio server", slog.String("error", err.Error()))
				os.Exit(1)
			}
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().String("host", "", "Specify the myshoes host")

	// Bind flag to viper
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("myshoes")
	viper.AutomaticEnv()
}

func initLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}),
	)
}

type runConfig struct {
	logger      *slog.Logger
	logCommands bool
}

type MyshoesMCPServer struct {
	logger *slog.Logger
	client *myshoesapi.Client
}

func runStdioServer(cfg runConfig) error {
	// Create myshoes client
	host := viper.GetString("host")
	if host == "" {
		return fmt.Errorf("host is required")
	}

	myshoesClient, err := myshoesapi.NewClient(host, http.DefaultClient, log.New(io.Discard, "", log.LstdFlags))
	if err != nil {
		return fmt.Errorf("failed to create myshoes client: %w", err)
	}

	myshoesClient.UserAgent = fmt.Sprintf("myshoes-mcp-server/%s", version)

	mms := MyshoesMCPServer{
		logger: cfg.logger,
		client: myshoesClient,
	}
	myshoesServer := server.NewMCPServer(
		"myshoes-mcp-server",
		"1.0.0",
	)
	tool := mcp.NewTool("list_target",
		mcp.WithDescription("List target from myshoes API"),
	)
	myshoesServer.AddTool(tool, mms.listTargetHandler)

	if err := server.ServeStdio(myshoesServer); err != nil {
		return fmt.Errorf("failed to serve stdio: %w", err)
	}

	return nil
}

func (mms MyshoesMCPServer) listTargetHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	targets, err := mms.client.ListTarget(ctx)
	if err != nil {
		mms.logger.Warn("failed to list targets", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}

	jb, err := json.Marshal(targets)
	if err != nil {
		mms.logger.Warn("failed to marshal targets", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to marshal targets: %w", err)
	}

	return mcp.NewToolResultText(string(jb)), nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
