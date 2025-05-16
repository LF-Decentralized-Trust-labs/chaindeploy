package serve

import (
	"context"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/config"
	_ "github.com/chainlaunch/chainlaunch/docs" // swagger docs
	"github.com/chainlaunch/chainlaunch/pkg/auth"
	backuphttp "github.com/chainlaunch/chainlaunch/pkg/backups/http"
	backupservice "github.com/chainlaunch/chainlaunch/pkg/backups/service"
	configservice "github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabrichandler "github.com/chainlaunch/chainlaunch/pkg/fabric/handler"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/handler"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	metricscommon "github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	"github.com/chainlaunch/chainlaunch/pkg/monitoring"
	nodeTypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"

	"github.com/chainlaunch/chainlaunch/pkg/audit"
	"github.com/chainlaunch/chainlaunch/pkg/metrics"
	networkshttp "github.com/chainlaunch/chainlaunch/pkg/networks/http"
	networksservice "github.com/chainlaunch/chainlaunch/pkg/networks/service"
	nodeshttp "github.com/chainlaunch/chainlaunch/pkg/nodes/http"
	nodesservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	notificationhttp "github.com/chainlaunch/chainlaunch/pkg/notifications/http"
	notificationservice "github.com/chainlaunch/chainlaunch/pkg/notifications/service"
	"github.com/chainlaunch/chainlaunch/pkg/plugin"
	settingshttp "github.com/chainlaunch/chainlaunch/pkg/settings/http"
	settingsservice "github.com/chainlaunch/chainlaunch/pkg/settings/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	httpSwagger "github.com/swaggo/http-swagger"
)

// var (
// 	port    int
// 	dbPath  string
// 	queries *db.Queries
// 	dev     bool
// 	// HTTP TLS configuration variables
// 	tlsCertFile string
// 	tlsKeyFile  string

// 	dataPath string
// )

// spaHandler implements the http.Handler interface for serving a Single Page Application
type spaHandler struct {
	indexPath  string
	fsys       embed.FS
	staticPath string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get the absolute path to prevent directory traversal
	path := filepath.Join(h.staticPath, strings.TrimPrefix(r.URL.Path, "/"))

	// Try to serve the requested file
	content, err := h.fsys.ReadFile(path)
	if err != nil {
		// If the file doesn't exist, serve index.html
		content, err = h.fsys.ReadFile(filepath.Join(h.staticPath, h.indexPath))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	w.Write(content)
}

// @title ChainLaunch API
// @version 1.0
// @description ChainLaunch API provides services for managing blockchain networks and cryptographic keys
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://chainlaunch.dev/support
// @contact.email support@chainlaunch.dev

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8100
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.basic BasicAuth
// @in header
// @name Authorization

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name session_id

// @tag.name Authentication
// @tag.description User authentication and authorization operations

// @tag.name Backup Schedules
// @tag.description Backup schedule configuration and management

// @tag.name Backup Targets
// @tag.description Backup target location configuration and management

// @tag.name Backups
// @tag.description Backup management operations

// @tag.name Besu Networks
// @tag.description Hyperledger Besu network management operations

// @tag.name Fabric Networks
// @tag.description Hyperledger Fabric network management operations

// @tag.name Keys
// @tag.description Cryptographic key management operations

// @tag.name Nodes
// @tag.description Network node management operations

// @tag.name Notifications
// @tag.description System notification configuration and management

// @tag.name Organizations
// @tag.description Organization management operations

// @tag.name Plugins
// @tag.description Plugin management operations

// @tag.name Providers
// @tag.description Key provider management operations

// @tag.name Settings
// @tag.description Settings management operations

// @tag.name Users
// @tag.description User account management operations

// Add these constants at the top level
const (
	keyLength         = 32 // 256 bits
	encryptionKeyFile = "encryption_key"
	sessionKeyFile    = "session_key"
)

// Add these new functions
func generateRandomKey(length int) ([]byte, error) {
	// Generate random bytes
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %v", err)
	}

	return key, nil
}

func getConfigDir(dataPath string) (string, error) {
	// If dataPath is provided, use it directly
	if dataPath != "" {
		return dataPath, nil
	}

	// Fallback to XDG_CONFIG_HOME
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "chainlaunch"), nil
	}

	// Then fallback to HOME
	home := os.Getenv("HOME")
	if home == "" {
		// Final fallback to user home dir
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not find home directory: %v", err)
		}
	}

	// Default fallback: ~/.chainlaunch
	return filepath.Join(home, "chainlaunch"), nil
}

func ensureKeyExists(filename string, dataPath string) (string, error) {
	// First check if the key is already set in environment
	envKey := strings.ToUpper(strings.TrimSuffix(filename, "_key"))
	if key := os.Getenv(envKey); key != "" {
		return key, nil
	}

	configDir, err := getConfigDir(dataPath)
	if err != nil {
		return "", err
	}

	keyPath := filepath.Join(configDir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %v", err)
	}

	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Generate new key
		key, err := generateRandomKey(keyLength)
		if err != nil {
			return "", err
		}

		// Encode key as hex string
		keyString := hex.EncodeToString(key)

		// Write key to file with restricted permissions
		if err := ioutil.WriteFile(keyPath, []byte(keyString), 0600); err != nil {
			return "", fmt.Errorf("failed to write key file: %v", err)
		}

		return keyString, nil
	}

	// Read existing key
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %v", err)
	}

	return string(keyBytes), nil
}

// setupServer configures and returns the HTTP server
func setupServer(queries *db.Queries, authService *auth.AuthService, views embed.FS, dev bool, dbPath string, dataPath string) *chi.Mux {
	// Initialize services
	keyManagementService, err := service.NewKeyManagementService(queries)
	if err != nil {
		log.Fatal("Failed to initialize key management service:", err)
	}
	if err := keyManagementService.InitializeKeyProviders(context.Background()); err != nil {
		log.Fatal("Failed to initialize key providers:", err)
	}
	configService := configservice.NewConfigService(dataPath)
	organizationService := fabricservice.NewOrganizationService(queries, keyManagementService, configService)
	logger := logger.NewDefault()

	auditService := audit.NewService(queries, 10)

	nodeEventService := nodesservice.NewNodeEventService(queries, logger)
	settingsService := settingsservice.NewSettingsService(queries, logger)
	_, err = settingsService.InitializeDefaultSettings(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize default settings: %v", err)
	}
	settingsHandler := settingshttp.NewHandler(settingsService, logger)

	// Initialize metrics service
	metricsConfig := metricscommon.DefaultConfig()
	nodesService := nodesservice.NewNodeService(queries, logger, keyManagementService, organizationService, nodeEventService, configService, settingsService)
	metricsService, err := metrics.NewService(metricsConfig, queries, nodesService)
	if err != nil {
		log.Fatal("Failed to initialize metrics service:", err)
	}
	nodesService.SetMetricsService(metricsService)
	metricsHandler := metrics.NewHandler(metricsService, logger)

	networksService := networksservice.NewNetworkService(queries, nodesService, keyManagementService, logger, organizationService)
	notificationService := notificationservice.NewNotificationService(queries, logger)
	backupService := backupservice.NewBackupService(queries, logger, notificationService, dbPath, configService)

	// Initialize and start monitoring service
	monitoringConfig := &monitoring.Config{
		DefaultCheckInterval:    1 * time.Minute,  // Check nodes every minute by default
		DefaultTimeout:          10 * time.Second, // 10 second timeout for checks
		DefaultFailureThreshold: 3,                // Alert after 3 consecutive failures
		Workers:                 3,                // Use 3 worker goroutines
	}
	monitoringService := monitoring.NewService(logger, monitoringConfig, notificationService, nodesService)

	// Start the monitoring service with a background context
	monitoringCtx, monitoringCancel := context.WithCancel(context.Background())
	if err := monitoringService.Start(monitoringCtx); err != nil {
		log.Fatal("Failed to start monitoring service:", err)
	}

	// Register shutdown handler for the monitoring service
	go func() {
		// This is a simple channel to catch SIGINT/SIGTERM
		// In a real app, you would tie this to your app's shutdown logic
		c := make(chan os.Signal, 1)
		<-c
		monitoringCancel()
		if err := monitoringService.Stop(); err != nil {
			log.Printf("Error stopping monitoring service: %v", err)
		}
	}()

	// Add nodes to monitor based on existing nodes in the system
	go func() {
		// Give other services time to initialize
		time.Sleep(5 * time.Second)

		// Get all nodes from the node service
		ctx := context.Background()
		var allNodes []nodesservice.NodeResponse
		fabricPlatform := nodeTypes.PlatformFabric
		nodes, err := nodesService.ListNodes(ctx, &fabricPlatform, 1, 100)
		if err != nil {
			log.Printf("Failed to fetch nodes for monitoring: %v", err)
			return
		}
		allNodes = append(allNodes, nodes.Items...)

		// Get Besu nodes
		besuPlatform := nodeTypes.PlatformBesu
		besuNodes, err := nodesService.ListNodes(ctx, &besuPlatform, 1, 100)
		if err != nil {
			log.Printf("Failed to fetch Besu nodes for monitoring: %v", err)
		} else {
			allNodes = append(allNodes, besuNodes.Items...)
			logger.Infof("Added %d Besu nodes for monitoring", len(besuNodes.Items))
		}

		// Add each node to monitoring
		for _, node := range allNodes {
			var monitorNode *monitoring.Node
			switch node.NodeType {
			case nodeTypes.NodeTypeFabricPeer:
				// Create a monitoring node from the node data
				monitorNode = &monitoring.Node{
					ID:               node.ID,
					Name:             node.Name,
					Endpoint:         node.Endpoint,
					Platform:         string(node.Platform),
					CheckInterval:    1 * time.Minute,
					Timeout:          10 * time.Second,
					FailureThreshold: 3,
				}
			case nodeTypes.NodeTypeFabricOrderer:
				// Create a monitoring node from the node data
				monitorNode = &monitoring.Node{
					ID:               node.ID,
					Name:             node.Name,
					Endpoint:         node.Endpoint,
					Platform:         string(node.Platform),
					CheckInterval:    1 * time.Minute,
					Timeout:          10 * time.Second,
					FailureThreshold: 3,
				}
			case nodeTypes.NodeTypeBesuFullnode:
				rcpEndpoint := fmt.Sprintf("%s:%d", node.BesuNode.RPCHost, node.BesuNode.RPCPort)
				// Create a monitoring node from the node data
				monitorNode = &monitoring.Node{
					ID:               node.ID,
					Name:             node.Name,
					Endpoint:         rcpEndpoint,
					Platform:         string(node.Platform),
					CheckInterval:    1 * time.Minute,
					Timeout:          10 * time.Second,
					FailureThreshold: 3,
				}
			default:
				logger.Infof("Skipping node %s (%s) as it is not a supported node type", node.Name, node.ID)
				continue
			}

			if monitoringService.NodeExists(node.ID) {
				logger.Infof("Node %s already exists in monitoring", node.Name)
				continue
			}

			if err := monitoringService.AddNode(monitorNode); err != nil {
				logger.Infof("Failed to add node %s to monitoring: %v", node.ID, err)
				continue
			}

			logger.Infof("Added node %s (%d) to monitoring", node.Name, node.ID)
		}
	}()

	// Initialize plugin store and manager
	pluginStore := plugin.NewSQLStore(queries)
	pluginManager, err := plugin.NewPluginManager(filepath.Join(dataPath, "plugins"))
	if err != nil {
		log.Fatal("Failed to initialize plugin manager:", err)
	}
	pluginHandler := plugin.NewHandler(pluginStore, pluginManager, logger)

	// Initialize handlers
	keyManagementHandler := handler.NewKeyManagementHandler(keyManagementService)
	organizationHandler := fabrichandler.NewOrganizationHandler(organizationService)
	nodesHandler := nodeshttp.NewNodeHandler(nodesService, logger)
	networksHandler := networkshttp.NewHandler(
		networksService,
		nodesService,
	)
	backupHandler := backuphttp.NewHandler(backupService)
	notificationHandler := notificationhttp.NewNotificationHandler(notificationService)
	authHandler := auth.NewHandler(authService)
	auditHandler := audit.NewHandler(auditService, logger)
	// Setup router
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Add CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes (no auth required)
		r.Post("/auth/login", response.Middleware(authHandler.LoginHandler))

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(authService))
			r.Use(audit.HTTPMiddleware(auditService)) // Add audit middleware

			// Mount auth routes
			authHandler.RegisterRoutes(r)

			// Mount key management routes
			keyManagementHandler.RegisterRoutes(r)
			// Mount organization routes
			organizationHandler.RegisterRoutes(r)
			// Mount nodes routes
			nodesHandler.RegisterRoutes(r)
			// Mount networks routes
			networksHandler.RegisterRoutes(r)
			// Mount backups routes
			backupHandler.RegisterRoutes(r)
			// Mount notifications routes
			notificationHandler.RegisterRoutes(r)
			// Mount settings routes
			settingsHandler.RegisterRoutes(r)
			// Mount plugin routes
			pluginHandler.RegisterRoutes(r)
			// Mount metrics routes
			metricsHandler.RegisterRoutes(r)

			// Mount audit routes
			auditHandler.RegisterRoutes(r)
		})
	})
	r.Get("/api/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/api/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))
	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))

	// Serve UI static files in production mode
	if !dev {
		spa := spaHandler{
			staticPath: "web/dist",
			indexPath:  "index.html",
			fsys:       views,
		}
		r.Handle("/*", spa)
	}

	return r
}

func runMigrations(database *sql.DB, migrationsFS embed.FS) error {
	driver, err := sqlite3.WithInstance(database, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("could not create sqlite driver: %v", err)
	}

	// Use embedded migrations instead of file system
	d, err := iofs.New(migrationsFS, "pkg/db/migrations")
	if err != nil {
		return fmt.Errorf("could not create iofs driver: %v", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs", d,
		"sqlite3", driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run migrations: %v", err)
	}

	return nil
}

type serveCmd struct {
	logger    *logger.Logger
	configCMD config.ConfigCMD

	port        int
	dbPath      string
	tlsCertFile string
	tlsKeyFile  string
	dataPath    string
	dev         bool

	queries *db.Queries
}

// validate validates the serve command configuration
func (c *serveCmd) validate() error {
	if c.port <= 0 || c.port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.port)
	}

	if c.dbPath == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	// If TLS is configured, both cert and key files must be provided
	if (c.tlsCertFile != "" && c.tlsKeyFile == "") || (c.tlsCertFile == "" && c.tlsKeyFile != "") {
		return fmt.Errorf("both TLS certificate and key files must be provided")
	}

	// If TLS files are provided, verify they exist
	if c.tlsCertFile != "" {
		if _, err := os.Stat(c.tlsCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate file not found: %s", c.tlsCertFile)
		}
	}
	if c.tlsKeyFile != "" {
		if _, err := os.Stat(c.tlsKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file not found: %s", c.tlsKeyFile)
		}
	}

	// Ensure data path exists or can be created
	if c.dataPath != "" {
		if err := os.MkdirAll(c.dataPath, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %v", err)
		}
	}

	return nil
}

func (c *serveCmd) preRun() error {
	// Ensure the database directory exists
	dbDir := filepath.Dir(c.dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Convert dataPath to absolute path if it's not empty
	if c.dataPath != "" {
		absPath, err := filepath.Abs(c.dataPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for data directory: %v", err)
		}
		c.dataPath = absPath
	}

	// Initialize database connection
	database, err := sql.Open("sqlite3", c.dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// Run migrations
	if err := runMigrations(database, c.configCMD.MigrationsFS); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create queries instance
	c.queries = db.New(database)

	return nil
}

func (c *serveCmd) run() error {
	// Initialize encryption key with dataPath
	encryptionKey, err := ensureKeyExists(encryptionKeyFile, c.dataPath)
	if err != nil {
		log.Fatalf("Failed to initialize encryption key: %v", err)
	}
	if err := os.Setenv("KEY_ENCRYPTION_KEY", encryptionKey); err != nil {
		log.Fatalf("Failed to set encryption key environment variable: %v", err)
	}

	// Initialize session key with dataPath
	sessionKey, err := ensureKeyExists(sessionKeyFile, c.dataPath)
	if err != nil {
		log.Fatalf("Failed to initialize session key: %v", err)
	}
	if err := os.Setenv("SESSION_ENCRYPTION_KEY", sessionKey); err != nil {
		log.Fatalf("Failed to set session key environment variable: %v", err)
	}

	c.logger.Infof("Starting server on port %d...", c.port)
	c.logger.Infof("Using database: %s", c.dbPath)
	if c.dev {
		c.logger.Info("Running in development mode")
	} else {
		c.logger.Info("Running in production mode")
	}

	// Initialize auth service with database
	authService := auth.NewAuthService(c.queries)

	// Check if any users exist
	users, err := authService.ListUsers(context.Background())
	if err != nil {
		log.Fatalf("Failed to check existing users: %v", err)
	}

	// Get environment variables
	username := os.Getenv("CHAINLAUNCH_USER")
	password := os.Getenv("CHAINLAUNCH_PASSWORD")

	if len(users) == 0 {
		// No users exist, check for required environment variables
		if username == "" || password == "" {
			log.Fatal("No users found in database. CHAINLAUNCH_USER and CHAINLAUNCH_PASSWORD environment variables must be set for initial user creation")
		}

		// Create initial user with provided credentials
		if _, err := authService.CreateUser(context.Background(), &auth.CreateUserRequest{
			Username: username,
			Password: password,
			Role:     auth.RoleAdmin,
		}); err != nil {
			log.Fatalf("Failed to create initial user: %v", err)
		}
		log.Printf("Created initial user with username: %s", username)
	} else if password != "" {
		user, err := authService.GetUserByUsername(username)
		if err != nil {
			log.Fatalf("Failed to get user: %v", err)
		}
		// If password is set and users exist, update the first user's password
		if err := authService.UpdateUserPassword(context.Background(), user.Username, password); err != nil {
			log.Fatalf("Failed to update user password: %v", err)
		}

		if _, err := authService.UpdateUser(context.Background(), user.ID, &auth.UpdateUserRequest{
			Role: auth.RoleAdmin,
		}); err != nil {
			log.Fatalf("Failed to update user role to admin: %v", err)
		}
		log.Printf("Updated password and role for user: %s", user.Username)
	}

	// Setup and start HTTP server
	router := setupServer(c.queries, authService, c.configCMD.Views, c.dev, c.dbPath, c.dataPath)

	// Start HTTP server in a goroutine
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.port),
		Handler: router,
	}

	// Check if TLS cert and key files exist
	if c.tlsCertFile != "" && c.tlsKeyFile != "" {
		c.logger.Infof("HTTPS server listening on :%d", c.port)
		err = httpServer.ListenAndServeTLS(c.tlsCertFile, c.tlsKeyFile)
	} else {
		c.logger.Infof("HTTP server listening on :%d", c.port)
		err = httpServer.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	return nil
}

func (c *serveCmd) postRun() error {
	// do nothing
	return nil
}

// Command returns the serve command
func Command(configCMD config.ConfigCMD, logger *logger.Logger) *cobra.Command {
	serveCmd := &serveCmd{
		configCMD: configCMD,
		logger:    logger,
	}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the API server",
		Long: `Start the HTTP API server on the specified port.
For example:
  chainlaunch serve --port 8100`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := serveCmd.validate(); err != nil {
				return err
			}
			return serveCmd.preRun()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveCmd.run()
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return serveCmd.postRun()
		},
	}

	// Add port flags
	cmd.Flags().IntVarP(&serveCmd.port, "port", "p", 8100, "Port to run the HTTP server on")

	// Add database path flag
	defaultDBPath := filepath.Join("data", "chainlaunch.db")
	cmd.Flags().StringVar(&serveCmd.dbPath, "db", defaultDBPath, "Path to SQLite database file")

	// Add HTTP TLS configuration flags
	cmd.Flags().StringVar(&serveCmd.tlsCertFile, "tls-cert", "", "Path to TLS certificate file for HTTP server (required)")
	cmd.Flags().StringVar(&serveCmd.tlsKeyFile, "tls-key", "", "Path to TLS key file for HTTP server (required)")

	// Update the default data path to use the OS-specific user config directory
	defaultDataPath := ""
	if configDir, err := os.UserConfigDir(); err == nil {
		defaultDataPath = filepath.Join(configDir, "chainlaunch")
	} else {
		// Fallback to home directory if UserConfigDir fails
		if homeDir, err := os.UserHomeDir(); err == nil {
			defaultDataPath = filepath.Join(homeDir, ".chainlaunch")
		}
	}

	cmd.Flags().StringVar(&serveCmd.dataPath, "data", defaultDataPath, "Path to data directory")
	// Add development mode flag
	cmd.Flags().BoolVar(&serveCmd.dev, "dev", false, "Run in development mode")

	return cmd
}
