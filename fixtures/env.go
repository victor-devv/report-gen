package fixtures

import (
	"database/sql"
	"fmt"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/store"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

type TestEnv struct {
	Config *config.Config
	Db     *sql.DB
}

func NewTestEnv(t *testing.T) *TestEnv {
	os.Setenv("ENV", string(config.Env_Test))
	conf, err := config.New()
	//assert that no error occurred
	require.NoError(t, err)

	db, err := store.NewPostgresDb(conf)

	return &TestEnv{
		Config: conf,
		Db:     db,
	}
}

func (te *TestEnv) SetupDb(t *testing.T) func(t *testing.T) {
	// create database tables
	m, err := migrate.New(
		fmt.Sprintf("file:///%s/migrations", te.Config.ProjectRoot),
		te.Config.DatabaseUrl())
	require.NoError(t, err)

	// throw error if no db changes (the db shouldd be empty at the start of the test)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}

	return te.TeardownDb
}

func (te *TestEnv) TeardownDb(t *testing.T) {
	_, err := te.Db.Exec(fmt.Sprintf("TRUNCATE TABLE %s", strings.Join([]string{
		"users",
		"refresh_token",
		"reports",
	}, ", ")))
	require.NoError(t, err)
}
