package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type signalingResponseWriter struct {
	gin.ResponseWriter
	wrote chan struct{}
	once  sync.Once
}

func (w *signalingResponseWriter) signalWrite() {
	w.once.Do(func() {
		close(w.wrote)
	})
}

func (w *signalingResponseWriter) Write(data []byte) (int, error) {
	w.signalWrite()
	return w.ResponseWriter.Write(data)
}

func (w *signalingResponseWriter) WriteString(data string) (int, error) {
	w.signalWrite()
	return w.ResponseWriter.WriteString(data)
}

func TestOaiResponsesStreamHandlerRecoversUsageAfterClientGone(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writer := &signalingResponseWriter{
		ResponseWriter: c.Writer,
		wrote:          make(chan struct{}),
	}
	c.Writer = writer

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestCtx)

	upstreamReader, upstreamWriter := io.Pipe()
	resp := &http.Response{Body: upstreamReader}
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-sol",
		},
	}

	writerDone := make(chan error, 1)
	go func() {
		if _, err := fmt.Fprintln(upstreamWriter, `data: {"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"query\":\"yunbay\"}"}}`); err != nil {
			writerDone <- err
			return
		}

		select {
		case <-writer.wrote:
		case <-time.After(2 * time.Second):
			writerDone <- fmt.Errorf("first response was not written")
			return
		}

		cancelRequest()
		time.Sleep(20 * time.Millisecond)

		if _, err := fmt.Fprintln(upstreamWriter, `data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.6-sol","usage":{"input_tokens":97984,"input_tokens_details":{"cached_tokens":91904},"output_tokens":182,"total_tokens":98166}}}`); err != nil {
			writerDone <- err
			return
		}
		if _, err := fmt.Fprintln(upstreamWriter, "data: [DONE]"); err != nil {
			writerDone <- err
			return
		}
		writerDone <- upstreamWriter.Close()
	}()

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.NoError(t, <-writerDone)

	assert.Equal(t, 97984, usage.PromptTokens)
	assert.Equal(t, 91904, usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 182, usage.CompletionTokens)
	assert.Equal(t, 98166, usage.TotalTokens)
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	assert.NotContains(t, recorder.Body.String(), "response.completed")
}
