package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShouldRetryAllowsRetryAfterChannelAffinityFailureIsHandled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("channel_affinity_skip_retry_on_failure", true)

	newAPIError := types.NewOpenAIError(
		assertionTestError("upstream temporarily unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		503,
	)

	require.False(t, shouldRetry(ctx, newAPIError, 1))

	service.HandleChannelAffinityFailure(ctx)

	require.True(t, shouldRetry(ctx, newAPIError, 1))
}

type assertionTestError string

func (e assertionTestError) Error() string {
	return string(e)
}

func TestGetChannelRetriesToAlternateChannelAfterAffinityChannelFailure(t *testing.T) {
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
	channelA := &model.Channel{
		Id:       301,
		Name:     "affinity-primary",
		Key:      "key-a",
		Status:   common.ChannelStatusEnabled,
		Group:    "default",
		Models:   "gpt-5",
		Priority: &priority,
	}
	channelB := &model.Channel{
		Id:       302,
		Name:     "fallback-secondary",
		Key:      "key-b",
		Status:   common.ChannelStatusEnabled,
		Group:    "default",
		Models:   "gpt-5",
		Priority: &priority,
	}

	for _, channel := range []*model.Channel{channelA, channelB} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     "gpt-5",
			ChannelId: channel.Id,
			Enabled:   true,
			Priority:  &priority,
			Weight:    0,
		}).Error)
	}

	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelA.Id)
	common.SetContextKey(ctx, constant.ContextKeyChannelName, channelA.Name)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, channelA.Type)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, channelA.Key)
	ctx.Set("use_channel", []string{fmt.Sprintf("%d", channelA.Id)})
	ctx.Set("channel_affinity_cache_key", "new-api:channel_affinity:v1:test-affinity")
	ctx.Set("channel_affinity_ttl_seconds", 60)
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	ctx.Set("channel_affinity_meta", map[string]any{})

	info := &relaycommon.RelayInfo{
		TokenGroup:      "default",
		UsingGroup:      "default",
		UserGroup:       "default",
		OriginModelName: "gpt-5",
		RequestURLPath:  "/v1/responses",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}

	channelFailure := types.NewOpenAIError(assertionTestError("primary channel failed"), types.ErrorCodeBadResponseStatusCode, 503)
	processChannelError(ctx, *types.NewChannelError(channelA.Id, channelA.Type, channelA.Name, false, channelA.Key, channelA.GetAutoBan()), channelFailure)

	require.True(t, shouldRetry(ctx, channelFailure, 1))

	retryParam := &service.RetryParam{
		Ctx:         ctx,
		TokenGroup:  "default",
		ModelName:   "gpt-5",
		RequestPath: "/v1/responses",
		Retry:       common.GetPointer(0),
	}

	nextChannel, channelErr := getChannel(ctx, info, retryParam)
	require.Nil(t, channelErr)
	require.NotNil(t, nextChannel)
	require.Equal(t, channelB.Id, nextChannel.Id)
}
