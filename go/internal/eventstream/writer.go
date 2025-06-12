package eventstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Writer interface {
	io.Writer
	io.Closer
	JSON(v interface{}) error
}

type WriterImpl struct {
	http.ResponseWriter
}

var _ Writer = (*WriterImpl)(nil)

func NewWriter(w http.ResponseWriter) Writer {
	writer := &WriterImpl{
		ResponseWriter: w,
	}
	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	return writer
}

func (w *WriterImpl) JSON(v interface{}) error {
	d, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.ResponseWriter.Write([]byte(fmt.Sprintf("data: %s\n\n", d)))
	return err
}

func (w *WriterImpl) Close() error {
	_, err := w.ResponseWriter.Write([]byte("data: [DONE]\n\n"))
	return err
}
