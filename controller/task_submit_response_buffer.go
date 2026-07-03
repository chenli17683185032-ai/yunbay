package controller

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

const taskSubmitResponseBufferNoWritten = -1

type taskSubmitResponseBuffer struct {
	original gin.ResponseWriter
	header   http.Header
	body     bytes.Buffer
	size     int
	status   int
}

var _ gin.ResponseWriter = (*taskSubmitResponseBuffer)(nil)

func newTaskSubmitResponseBuffer(original gin.ResponseWriter) *taskSubmitResponseBuffer {
	w := &taskSubmitResponseBuffer{original: original}
	w.Reset()
	return w
}

func (w *taskSubmitResponseBuffer) Header() http.Header {
	return w.header
}

func (w *taskSubmitResponseBuffer) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.Written() {
			return
		}
		w.status = code
	}
}

func (w *taskSubmitResponseBuffer) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
	}
}

func (w *taskSubmitResponseBuffer) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.body.Write(data)
	w.size += n
	return n, err
}

func (w *taskSubmitResponseBuffer) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	n, err := io.WriteString(&w.body, s)
	w.size += n
	return n, err
}

func (w *taskSubmitResponseBuffer) Status() int {
	return w.status
}

func (w *taskSubmitResponseBuffer) Size() int {
	return w.size
}

func (w *taskSubmitResponseBuffer) Written() bool {
	return w.size != taskSubmitResponseBufferNoWritten
}

func (w *taskSubmitResponseBuffer) Flush() {
	w.WriteHeaderNow()
}

func (w *taskSubmitResponseBuffer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.size < 0 {
		w.size = 0
	}
	return w.original.Hijack()
}

func (w *taskSubmitResponseBuffer) CloseNotify() <-chan bool {
	return w.original.CloseNotify()
}

func (w *taskSubmitResponseBuffer) Pusher() http.Pusher {
	return w.original.Pusher()
}

func (w *taskSubmitResponseBuffer) FlushToOriginal() error {
	dst := w.original.Header()
	for key, values := range w.header {
		dst[key] = append([]string(nil), values...)
	}

	if w.Written() {
		w.original.WriteHeader(w.status)
		_, err := w.original.Write(w.body.Bytes())
		return err
	}

	if len(w.header) > 0 || w.status != http.StatusOK {
		w.original.WriteHeader(w.status)
	}
	return nil
}

func (w *taskSubmitResponseBuffer) Reset() {
	w.header = make(http.Header)
	w.body.Reset()
	w.size = taskSubmitResponseBufferNoWritten
	w.status = http.StatusOK
}
