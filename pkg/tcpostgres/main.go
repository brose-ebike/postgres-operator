package tcpostgres

import (
	"context"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// postgresContainer represents the postgres container type used in the module
type PostgresContainer struct {
	container testcontainers.Container
	username  string
	password  string
	database  string
}

type PostgresRequest struct {
	username string
	password string
	database string
}

type PostgresContainerOption func(tcReq *testcontainers.ContainerRequest, pgReq *PostgresRequest)

func WithInitialDatabase(user string, password string, dbName string) func(tcReq *testcontainers.ContainerRequest, pgReq *PostgresRequest) {
	return func(tcReq *testcontainers.ContainerRequest, pgReq *PostgresRequest) {
		// Update testcontainers request
		tcReq.Env["POSTGRES_USER"] = user
		tcReq.Env["POSTGRES_PASSWORD"] = password
		tcReq.Env["POSTGRES_DB"] = dbName
		// Update postgres request
		pgReq.username = user
		pgReq.password = password
		pgReq.database = dbName
	}
}

// setupPostgres creates an instance of the postgres container type
func SetupPostgres(ctx context.Context, opts ...PostgresContainerOption) (*PostgresContainer, error) {
	// Testcontainer Request
	tcReq := testcontainers.ContainerRequest{
		Image:        "postgres:14-alpine",
		Env:          map[string]string{},
		ExposedPorts: []string{"5432"},
		Cmd:          []string{"postgres", "-c", "fsync=off"},
		WaitingFor: wait.ForAll(
			wait.
				ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5 * time.Second),
		).WithDeadline(1 * time.Minute),
	}
	// Postgres Request
	pgReq := PostgresRequest{}

	// Handle options
	for _, opt := range opts {
		opt(&tcReq, &pgReq)
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: tcReq,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	return &PostgresContainer{
		container: container,
		username:  pgReq.username,
		password:  pgReq.password,
		database:  pgReq.database,
	}, nil
}

func (pgc *PostgresContainer) Terminate(ctx context.Context) error {
	if pgc.container == nil {
		return nil
	}
	return pgc.container.Terminate(ctx)
}

func (pgc *PostgresContainer) Hostname(ctx context.Context) (string, error) {
	return pgc.container.Host(ctx)
}

func (pgc *PostgresContainer) Port(ctx context.Context) (int, error) {
	// Convert Port Number to Port Object
	postgresPort, err := nat.NewPort("tcp", "5432")
	if err != nil {
		return 0, err
	}

	// Get Mapped Port
	containerPort, err := pgc.container.MappedPort(ctx, postgresPort)
	if err != nil {
		return 0, err
	}

	// Return port
	return containerPort.Int(), nil
}

func (pgc *PostgresContainer) Username() string {
	return pgc.username
}

func (pgc *PostgresContainer) Password() string {
	return pgc.password
}

func (pgc *PostgresContainer) Database() string {
	return pgc.database
}
