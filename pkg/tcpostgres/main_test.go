package tcpostgres

import (
	"context"
	"testing"
)

func TestPostgresTestContainer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Setup container
	container, err := SetupPostgres(ctx, WithInitialDatabase("pgtest", "pgtest", "postgres"))
	if err != nil {
		t.Errorf("Unable to setup postgres container")
	}
	// Cleanup container
	err = container.Terminate(ctx)
	if err != nil {
		t.Errorf("Unable to terminate postgres container")
	}
}
