package dimse_test

import (
	"context"
	"testing"
	"time"

	"github.com/otcheredev/ris-dicom-connector/pkg/dimse"
)

func TestCEcho(t *testing.T) {
	// Test against Orthanc
	config := dimse.AssociationConfig{
		Host:       "localhost",
		Port:       4242,
		CallingAET: "DICOM_CONNECTOR",
		CalledAET:  "ORTHANC",
		Timeout:    10 * time.Second,
	}

	assoc := dimse.NewAssociation(config)
	defer assoc.Close()

	ctx := context.Background()

	// Test C-ECHO
	err := assoc.CEcho(ctx)
	if err != nil {
		t.Fatalf("C-ECHO failed: %v", err)
	}

	t.Log("C-ECHO successful!")
}

func TestConnectionPool(t *testing.T) {
	poolConfig := dimse.PoolConfig{
		AssociationConfig: dimse.AssociationConfig{
			Host:       "localhost",
			Port:       4242,
			CallingAET: "DICOM_CONNECTOR",
			CalledAET:  "ORTHANC",
			Timeout:    10 * time.Second,
		},
		MaxPoolSize: 3,
		MaxIdleTime: 1 * time.Minute,
	}

	pool := dimse.NewConnectionPool(poolConfig)
	defer pool.Close()

	ctx := context.Background()

	// Get connection from pool
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Test C-ECHO
	err = conn.CEcho(ctx)
	if err != nil {
		t.Fatalf("C-ECHO failed: %v", err)
	}

	// Return to pool
	pool.Put(conn)

	// Check stats
	stats := pool.Stats()
	if stats.TotalConnections != 1 {
		t.Errorf("Expected 1 connection in pool, got %d", stats.TotalConnections)
	}

	t.Log("Connection pool test successful!")
}
