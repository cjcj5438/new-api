package service

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCacheGetRandomSatisfiedChannelSkipsPreviouslyFailedChannelsWithinSamePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousMainDBType := common.MainDatabaseType()
	previousLogDBType := common.LogDatabaseType()

	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetDatabaseTypes(previousMainDBType, previousLogDBType)
	})

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	priority := int64(0)
	channels := []*model.Channel{
		{Id: 101, Name: "channel-a", Key: "key-a", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o-mini", Priority: &priority},
		{Id: 102, Name: "channel-b", Key: "key-b", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o-mini", Priority: &priority},
	}
	for _, channel := range channels {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     "gpt-4o-mini",
			ChannelId: channel.Id,
			Enabled:   true,
			Priority:  &priority,
			Weight:    0,
		}).Error)
	}

	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("use_channel", []string{"101"})

	retryParam := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "default",
		ModelName:   "gpt-4o-mini",
		RequestPath: "/v1/chat/completions",
		Retry:       common.GetPointer(0),
	}

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(retryParam)

	require.NoError(t, err)
	require.Equal(t, "default", selectedGroup)
	require.NotNil(t, channel)
	require.Equal(t, 102, channel.Id)
}

func TestCacheGetRandomSatisfiedChannelPrefersRemainingHighestPriorityChannelBeforeLowerPriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousMainDBType := common.MainDatabaseType()
	previousLogDBType := common.LogDatabaseType()

	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetDatabaseTypes(previousMainDBType, previousLogDBType)
	})

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	highPriority := int64(10)
	lowPriority := int64(0)
	channels := []struct {
		id       int
		priority *int64
	}{
		{id: 201, priority: &highPriority},
		{id: 202, priority: &highPriority},
		{id: 203, priority: &lowPriority},
	}

	for _, item := range channels {
		require.NoError(t, db.Create(&model.Channel{
			Id:       item.id,
			Name:     fmt.Sprintf("channel-%d", item.id),
			Key:      fmt.Sprintf("key-%d", item.id),
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-4o-mini",
			Priority: item.priority,
		}).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     "gpt-4o-mini",
			ChannelId: item.id,
			Enabled:   true,
			Priority:  item.priority,
			Weight:    0,
		}).Error)
	}

	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("use_channel", []string{"201"})

	retryParam := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "default",
		ModelName:   "gpt-4o-mini",
		RequestPath: "/v1/chat/completions",
		Retry:       common.GetPointer(1),
	}

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(retryParam)

	require.NoError(t, err)
	require.Equal(t, "default", selectedGroup)
	require.NotNil(t, channel)
	require.Equal(t, 202, channel.Id)
}
