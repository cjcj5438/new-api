package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperResponsesCompactPreservesOriginModelName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("model_mapping", `{"gpt-5.4-mini":"gpt-5.4"}`)

	requestedModel := ratio_setting.WithCompactModelSuffix("gpt-5.4-mini")
	request := &dto.OpenAIResponsesRequest{Model: requestedModel}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: requestedModel,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: requestedModel},
	}

	err := ModelMappedHelper(ctx, info, request)

	require.NoError(t, err)
	require.True(t, info.IsModelMapped)
	require.Equal(t, requestedModel, info.OriginModelName)
	require.Equal(t, "gpt-5.4", info.UpstreamModelName)
	require.Equal(t, "gpt-5.4", request.Model)
}
