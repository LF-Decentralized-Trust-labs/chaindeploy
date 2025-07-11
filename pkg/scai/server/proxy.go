package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// StartSNIProxy starts a TLS proxy server that routes connections based on SNI slug.
// lookupBackend(slug) should return the backend address (host:port) for the given slug.
func StartSNIProxy(addr, certFile, keyFile string, lookupBackend func(slug string) (string, error), logger *logger.Logger) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS cert/key: %w", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			// Accept all SNI, but log it
			logger.Info("TLS handshake", "server_name", chi.ServerName)
			return nil, nil
		},
	}
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	logger.Info("SNI proxy listening", "address", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("accept failed", "error", err)
			continue
		}
		go handleSNIConn(conn, lookupBackend, logger)
	}
}

func handleSNIConn(conn net.Conn, lookupBackend func(slug string) (string, error), logger *logger.Logger) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		logger.Error("connection is not TLS")
		conn.Close()
		return
	}
	var closeOnce sync.Once
	if err := tlsConn.Handshake(); err != nil {
		logger.Error("TLS handshake failed", "error", err)
		return
	}
	sni := tlsConn.ConnectionState().ServerName
	if sni == "" {
		logger.Error("no SNI provided")
		return
	}
	// Parse slug from SNI: expect {slug}.{domain}
	parts := strings.SplitN(sni, ".", 2)
	if len(parts) < 2 {
		logger.Error("invalid SNI format", "sni", sni)
		return
	}
	slug := parts[0]
	backendAddr, err := lookupBackend(slug)
	if err != nil {
		logger.Error("backend lookup failed", "slug", slug, "error", err)
		return
	}
	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		logger.Error("failed to connect to backend", "backend", backendAddr, "error", err)
		return
	}
	// Use a mutex to guard closing both connections
	var mu sync.Mutex
	closeBoth := func() {
		mu.Lock()
		defer mu.Unlock()
		closeOnce.Do(func() {
			backendConn.Close()
			conn.Close()
		})
	}
	logger.Info("proxying connection", "slug", slug, "backend", backendAddr)
	// Proxy data in both directions using goroutines and waitgroup
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(backendConn, tlsConn)
		closeBoth()
	}()
	go func() {
		defer wg.Done()
		io.Copy(tlsConn, backendConn)
		closeBoth()
	}()
	wg.Wait()
}
