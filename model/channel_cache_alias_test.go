package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitChannelCacheUsesAbilitiesForAliasModels(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldDB := DB
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		DB = oldDB
		channelSyncLock.Lock()
		group2model2channels = nil
		channelsIDM = nil
		channelSyncLock.Unlock()
	})

	common.MemoryCacheEnabled = true
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, db.Create(&Channel{Id: 9101, Status: common.ChannelStatusEnabled, Models: "glm5.2-free", Group: "default"}).Error)
	require.NoError(t, db.Create(&Ability{Group: "default", Model: "z-ai/glm-5.2", ChannelId: 9101, Enabled: true}).Error)

	InitChannelCache()

	channel, err := GetRandomSatisfiedChannel("default", "z-ai/glm-5.2", 0)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 9101, channel.Id)
}
