package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/goccy/go-json"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	myshoesapi "github.com/whywaita/myshoes/api/myshoes"
	"github.com/whywaita/myshoes/pkg/web"
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

// GetTargetInput is the input parameter for get_target tool.
type GetTargetInput struct {
	TargetID string `json:"target_id" jsonschema:"description=The UUID of the target"`
}

// CreateTargetInput is the input parameter for create_target tool.
type CreateTargetInput struct {
	Scope        string `json:"scope" jsonschema:"description=Repository (:owner/:repo) or organization (:org) scope"`
	ResourceType string `json:"resource_type" jsonschema:"description=Resource type (nano/micro/small/medium/large/xlarge/2xlarge/3xlarge/4xlarge)"`
	ProviderURL  string `json:"provider_url,omitempty" jsonschema:"description=Provider URL (optional)"`
	RunnerUser   string `json:"runner_user,omitempty" jsonschema:"description=Runner user (optional)"`
}

// UpdateTargetInput is the input parameter for update_target tool.
type UpdateTargetInput struct {
	TargetID     string `json:"target_id" jsonschema:"description=The UUID of the target to update"`
	ResourceType string `json:"resource_type,omitempty" jsonschema:"description=Resource type (nano/micro/small/medium/large/xlarge/2xlarge/3xlarge/4xlarge)"`
	ProviderURL  string `json:"provider_url,omitempty" jsonschema:"description=Provider URL"`
}

// DeleteTargetInput is the input parameter for delete_target tool.
type DeleteTargetInput struct {
	TargetID string `json:"target_id" jsonschema:"description=The UUID of the target to delete"`
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

	// Create MCP server using the official SDK
	myshoesServer := mcp.NewServer(&mcp.Implementation{
		Name:    "myshoes-mcp-server",
		Version: "1.0.0",
	}, nil)

	// Add the list_target tool
	myshoesServer.AddTool(&mcp.Tool{
		Name:        "list_target",
		Description: "List target from myshoes API",
	}, mms.listTargetHandler)

	// Add the get_target tool
	mcp.AddTool(myshoesServer, &mcp.Tool{
		Name:        "get_target",
		Description: "Get a target from myshoes API by ID",
	}, mms.getTargetHandler)

	// Add the create_target tool
	mcp.AddTool(myshoesServer, &mcp.Tool{
		Name:        "create_target",
		Description: "Create a new target in myshoes API",
	}, mms.createTargetHandler)

	// Add the update_target tool
	mcp.AddTool(myshoesServer, &mcp.Tool{
		Name:        "update_target",
		Description: "Update an existing target in myshoes API",
	}, mms.updateTargetHandler)

	// Add the delete_target tool
	mcp.AddTool(myshoesServer, &mcp.Tool{
		Name:        "delete_target",
		Description: "Delete a target from myshoes API",
	}, mms.deleteTargetHandler)

	// Start stdio transport
	var transport mcp.Transport = &mcp.StdioTransport{}
	if cfg.logCommands {
		transport = &mcp.LoggingTransport{
			Transport: transport,
			Writer:    os.Stderr,
		}
	}

	// Run the server
	return myshoesServer.Run(context.Background(), transport)
}

func (mms MyshoesMCPServer) listTargetHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Return the result with text content
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jb),
			},
		},
	}, nil
}

func (mms MyshoesMCPServer) getTargetHandler(ctx context.Context, _ *mcp.CallToolRequest, input GetTargetInput) (*mcp.CallToolResult, any, error) {
	target, err := mms.client.GetTarget(ctx, input.TargetID)
	if err != nil {
		mms.logger.Warn("failed to get target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to get target: %w", err)
	}

	jb, err := json.Marshal(target)
	if err != nil {
		mms.logger.Warn("failed to marshal target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to marshal target: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jb),
			},
		},
	}, nil, nil
}

func (mms MyshoesMCPServer) createTargetHandler(ctx context.Context, _ *mcp.CallToolRequest, input CreateTargetInput) (*mcp.CallToolResult, any, error) {
	param := web.TargetCreateParam{}
	param.Scope = input.Scope

	quoted := fmt.Sprintf(`"%s"`, input.ResourceType)
	if err := json.Unmarshal([]byte(quoted), &param.ResourceType); err != nil {
		return nil, nil, fmt.Errorf("invalid resource_type %q: %w", input.ResourceType, err)
	}

	if input.ProviderURL != "" {
		param.ProviderURL = &input.ProviderURL
	}
	if input.RunnerUser != "" {
		param.RunnerUser = &input.RunnerUser
	}

	target, err := mms.client.CreateTarget(ctx, param)
	if err != nil {
		mms.logger.Warn("failed to create target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to create target: %w", err)
	}

	jb, err := json.Marshal(target)
	if err != nil {
		mms.logger.Warn("failed to marshal target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to marshal target: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jb),
			},
		},
	}, nil, nil
}

func (mms MyshoesMCPServer) updateTargetHandler(ctx context.Context, _ *mcp.CallToolRequest, input UpdateTargetInput) (*mcp.CallToolResult, any, error) {
	param := web.TargetCreateParam{}

	if input.ResourceType != "" {
		quoted := fmt.Sprintf(`"%s"`, input.ResourceType)
		if err := json.Unmarshal([]byte(quoted), &param.ResourceType); err != nil {
			return nil, nil, fmt.Errorf("invalid resource_type %q: %w", input.ResourceType, err)
		}
	}

	if input.ProviderURL != "" {
		param.ProviderURL = &input.ProviderURL
	}

	target, err := mms.client.UpdateTarget(ctx, input.TargetID, param)
	if err != nil {
		mms.logger.Warn("failed to update target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to update target: %w", err)
	}

	jb, err := json.Marshal(target)
	if err != nil {
		mms.logger.Warn("failed to marshal target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to marshal target: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jb),
			},
		},
	}, nil, nil
}

func (mms MyshoesMCPServer) deleteTargetHandler(ctx context.Context, _ *mcp.CallToolRequest, input DeleteTargetInput) (*mcp.CallToolResult, any, error) {
	if err := mms.client.DeleteTarget(ctx, input.TargetID); err != nil {
		mms.logger.Warn("failed to delete target", slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to delete target: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully deleted target %s", input.TargetID),
			},
		},
	}, nil, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
