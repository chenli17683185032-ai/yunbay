package model

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogStatTestDB(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		initCol()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}

	DB = db
	LOG_DB = db
	initCol()
}

func TestSumUsedQuotaEmptyLogsReturnsZeroStats(t *testing.T) {
	setupLogStatTestDB(t)

	stat, err := SumUsedQuota(LogTypeUnknown, 0, 0, "", "", "", 0, "")
	if err != nil {
		t.Fatalf("SumUsedQuota returned error for empty logs: %v", err)
	}

	if stat.Quota != 0 || stat.Rpm != 0 || stat.Tpm != 0 {
		t.Fatalf("empty stats = %+v, want all zero", stat)
	}
}

func TestSumUsedQuotaNoRecentConsumeLogsReturnsZeroRpmTpm(t *testing.T) {
	setupLogStatTestDB(t)

	oldConsume := Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        time.Now().Add(-2 * time.Minute).Unix(),
		Type:             LogTypeConsume,
		Quota:            25,
		PromptTokens:     10,
		CompletionTokens: 15,
	}
	if err := LOG_DB.Create(&oldConsume).Error; err != nil {
		t.Fatalf("insert old consume log: %v", err)
	}

	stat, err := SumUsedQuota(LogTypeUnknown, 0, 0, "", "", "", 0, "")
	if err != nil {
		t.Fatalf("SumUsedQuota returned error when recent rpm/tpm window is empty: %v", err)
	}

	if stat.Quota != 25 {
		t.Fatalf("quota = %d, want 25", stat.Quota)
	}
	if stat.Rpm != 0 || stat.Tpm != 0 {
		t.Fatalf("recent rpm/tpm = rpm %d tpm %d, want 0/0", stat.Rpm, stat.Tpm)
	}
}

func TestSumUsedQuotaAggregatesCoalesceNullableSums(t *testing.T) {
	content, err := os.ReadFile("log.go")
	if err != nil {
		t.Fatalf("read log.go: %v", err)
	}
	source := string(content)
	for _, want := range []string{
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS tpm",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("SumUsedQuota aggregate SQL missing %q", want)
		}
	}
}
