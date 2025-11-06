package dimse

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConnectionPool manages a pool of DICOM associations
type ConnectionPool struct {
	config        AssociationConfig
	maxSize       int
	maxIdleTime   time.Duration
	connections   []*Association
	mu            sync.Mutex
	cleanupTicker *time.Ticker
	done          chan struct{}
}

// PoolConfig holds configuration for connection pool
type PoolConfig struct {
	AssociationConfig
	MaxPoolSize int
	MaxIdleTime time.Duration
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config PoolConfig) *ConnectionPool {
	if config.MaxPoolSize == 0 {
		config.MaxPoolSize = 5
	}
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 5 * time.Minute
	}

	pool := &ConnectionPool{
		config:        config.AssociationConfig,
		maxSize:       config.MaxPoolSize,
		maxIdleTime:   config.MaxIdleTime,
		connections:   make([]*Association, 0, config.MaxPoolSize),
		cleanupTicker: time.NewTicker(1 * time.Minute),
		done:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go pool.cleanup()

	return pool
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (*Association, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to find an idle connection
	for i, conn := range p.connections {
		if conn.IsConnected() {
			// Remove from pool
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			return conn, nil
		}
	}

	// Create new connection if pool not full
	if len(p.connections) < p.maxSize {
		conn := NewAssociation(p.config)
		if err := conn.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to create new connection: %w", err)
		}
		return conn, nil
	}

	return nil, fmt.Errorf("connection pool exhausted")
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn *Association) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Only return healthy connections to pool
	if !conn.IsConnected() {
		conn.Close()
		return
	}

	// Don't exceed max pool size
	if len(p.connections) >= p.maxSize {
		conn.Close()
		return
	}

	p.connections = append(p.connections, conn)
}

// Close closes all connections and stops the pool
func (p *ConnectionPool) Close() error {
	close(p.done)
	p.cleanupTicker.Stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	var errors []error
	for _, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	p.connections = nil

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while closing pool", len(errors))
	}

	return nil
}

// cleanup periodically removes idle connections
func (p *ConnectionPool) cleanup() {
	for {
		select {
		case <-p.cleanupTicker.C:
			p.removeIdleConnections()
		case <-p.done:
			return
		}
	}
}

// removeIdleConnections removes connections that have been idle too long
func (p *ConnectionPool) removeIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	active := make([]*Association, 0, len(p.connections))

	for _, conn := range p.connections {
		if now.Sub(conn.GetLastUsed()) > p.maxIdleTime {
			conn.Close()
		} else if conn.IsConnected() {
			active = append(active, conn)
		} else {
			conn.Close()
		}
	}

	p.connections = active
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return PoolStats{
		TotalConnections: len(p.connections),
		MaxSize:          p.maxSize,
	}
}

// PoolStats holds pool statistics
type PoolStats struct {
	TotalConnections int
	MaxSize          int
}
