package main

import (
	"context"
	"crypto/tls"
	"flag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/patrickmn/go-cache"
)

type Config struct {
	ServerPort          string
	UseTLS              bool
	CertFilePath        string
	KeyFilePath         string
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	CacheEvictionPeriod time.Duration
}

var (
	mu           sync.RWMutex
	certificates *tls.Certificate
	logger       *zap.SugaredLogger
)

func init() {
	// Create a logger
	rawLogger, _ := zap.NewProduction()
	logger = rawLogger.Sugar()
}

func loadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadCertificates() {
	for {
		cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
		if err != nil {
			log.Println("Failed to load key pair", err)
		} else {
			mu.Lock()
			certificates = &cert
			mu.Unlock()
		}

		time.Sleep(time.Minute)
	}

}

func getCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	mu.RLock()
	defer mu.RUnlock()
	return certificates, nil
}

func main() {
	defer logger.Sync() // Flush the logger before the application exits

	// Parse command line arguments
	configFile := flag.String("config", "/etc/dockerProxy/config", "Path to the config file")
	flag.Parse()

	// Load the configuration from the specified file
	config, err := loadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %s", err)
	}

	// Log the loaded configuration
	logger.Infow("Loaded configuration", "config", config)

	// Start a goroutine to periodically load new certificates
	if config.UseTLS {
		go loadCertificates()
	}

	// Create the cache for manifests
	manifestCache := cache.New(config.CacheEvictionPeriod, 1*time.Minute)

	// Create the HTTP handler
	handler := NewHandler(manifestCache, logger)

	server := &http.Server{
		Addr: config.ServerPort,
		TLSConfig: &tls.Config{
			GetCertificate: getCertificate,
		},
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
		Handler:      handler,
	}

	// Create a channel to receive the SIGTERM signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Start the server
		if config.UseTLS {
			log.Fatal(server.ListenAndServeTLS("", ""))
		} else {
			log.Fatal(server.ListenAndServe())
		}
	}()

	// Wait for the SIGTERM signal
	<-stop

	// Received SIGTERM, shut down gracefully
	logger.Info("Shutting down gracefully...")

	// Create a deadline to wait for currently served requests to complete
	deadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// Shut down the server
	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Failed to shut down server: %s", err)
	}

	logger.Info("Server shut down successfully")
}
