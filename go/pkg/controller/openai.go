package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jc-lab/distworker/go/internal/eventstream"
	"github.com/jc-lab/distworker/go/internal/openai"
	"github.com/jc-lab/distworker/go/pkg/api"
	"io"
	"log/slog"
	"net/http"
)

func (s *Server) openaiGenerateHandler(c *gin.Context) {
	var req openai.ChatCompletionRequest

	createTaskRequest := &api.CreateTaskRequest{
		Queue: s.config.Server.API.OpenAi.CompletionsQueue,
	}

	if _, err := doubleDecode(c.Request.Body, &req, &createTaskRequest.Input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, listener, err := s.CreateTask(c.Request.Context(), createTaskRequest, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.Stream {
		progressCh := listener.Progress(c.Request.Context())
		sseCh := make(chan any)

		go func() {
			defer close(sseCh)
			for progress := range progressCh {
				sseCh <- progress.Data
			}
		}()

		openaiStreamResponse(c, sseCh)
	} else {
		task, err = listener.Wait(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, task.Result)
	}
}

func (s *Server) openaiEmbedHandler(c *gin.Context) {
	var req openai.EmbedRequest

	createTaskRequest := &api.CreateTaskRequest{
		Queue: s.config.Server.API.OpenAi.EmbeddingsQueue,
	}

	if _, err := doubleDecode(c.Request.Body, &req, &createTaskRequest.Input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, listener, err := s.CreateTask(c.Request.Context(), createTaskRequest, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err = listener.Wait(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task.Result)
}

func (s *Server) openaiChatHandler(c *gin.Context) {
	var req openai.ChatCompletionRequest

	createTaskRequest := &api.CreateTaskRequest{
		Queue: s.config.Server.API.OpenAi.ChatCompletionsQueue,
	}

	if _, err := doubleDecode(c.Request.Body, &req, &createTaskRequest.Input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, listener, err := s.CreateTask(c.Request.Context(), createTaskRequest, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.Stream {
		progressCh := listener.Progress(c.Request.Context())
		sseCh := make(chan any)

		go func() {
			defer close(sseCh)
			for progress := range progressCh {
				sseCh <- progress.Data
			}
		}()

		openaiStreamResponse(c, sseCh)
	} else {
		task, err = listener.Wait(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, task.Result)
	}
}

func doubleDecode(r io.Reader, out1 interface{}, out2 interface{}) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, out1); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, out2); err != nil {
		return nil, err
	}
	return raw, nil
}

func openaiStreamResponse(c *gin.Context, ch chan any) {
	c.Header("Content-Type", "application/x-ndjson")
	sw := eventstream.NewWriter(c.Writer)

	c.Stream(func(w io.Writer) bool {
		val, ok := <-ch
		if !ok {
			_ = sw.Close()
			return false
		}
		if err := sw.JSON(val); err != nil {
			slog.Info(fmt.Sprintf("openaiStreamResponse: json.Marshal failed with %s", err))
			return false
		}

		return true
	})
}
