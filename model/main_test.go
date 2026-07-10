package model

import (
	"io"
	"log"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"
)

func TestInitDBWithoutMigrationsWithLoggerUsesProvidedLogger(t *testing.T) {
	oldDB := DB
	oldLogDB := LOG_DB
	oldSQLitePath := common.SQLitePath
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		_ = CloseDB()
		DB = oldDB
		LOG_DB = oldLogDB
		common.SQLitePath = oldSQLitePath
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})
	common.SQLitePath = filepath.Join(t.TempDir(), "custom-logger.db")
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	DB = nil
	LOG_DB = nil
	t.Setenv("SQL_DSN", "local")
	customLogger := gormlogger.New(log.New(io.Discard, "", 0), gormlogger.Config{LogLevel: gormlogger.Silent})

	require.NoError(t, InitDBWithoutMigrationsWithLogger(customLogger))
	require.Same(t, customLogger, DB.Logger)
	require.Same(t, DB, LOG_DB)
}

func TestInitDBWithoutMigrationsOpensSQLiteWithoutCreatingBusinessTables(t *testing.T) {
	oldDB := DB
	oldLogDB := LOG_DB
	oldSQLitePath := common.SQLitePath
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		if DB != nil && DB != oldDB {
			if sqlDB, err := DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.SQLitePath = oldSQLitePath
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})

	common.SQLitePath = filepath.Join(t.TempDir(), "preview-only.db")
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Setenv("SQL_DSN", "local")

	require.NoError(t, InitDBWithoutMigrations())
	require.NotNil(t, DB)
	require.Same(t, DB, LOG_DB)
	require.False(t, DB.Migrator().HasTable(&User{}))
	require.False(t, DB.Migrator().HasTable(&SubscriptionPlan{}))
	require.False(t, DB.Migrator().HasTable(&UserSubscription{}))
	require.False(t, DB.Migrator().HasTable(&ValuePackageQuotaMigrationReceipt{}))
	require.NotPanics(t, func() {
		require.NoError(t, CloseDB())
	})
}

func TestCloseDBAllowsUninitializedConnections(t *testing.T) {
	oldDB := DB
	oldLogDB := LOG_DB
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})
	DB = nil
	LOG_DB = nil

	require.NotPanics(t, func() {
		require.NoError(t, CloseDB())
	})
}
