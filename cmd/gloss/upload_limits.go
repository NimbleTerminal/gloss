package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

var errUploadQuota = errors.New("session upload quota exceeded")
var uploadIdleTimeout = 30 * time.Second

// Reserve each chunk before it can reach disk. stored includes in-flight
// uploads and exports; unsuccessful files refund every byte they reserved.
type uploadQuotaReader struct {
	s       *server
	reader  io.Reader
	charged int64
}

func (r *uploadQuotaReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n == 0 {
		return n, err
	}
	for {
		used := r.s.stored.Load()
		if int64(n) > maxSessionBytes-used {
			return 0, errUploadQuota
		}
		if r.s.stored.CompareAndSwap(used, used+int64(n)) {
			break
		}
	}
	r.charged += int64(n)
	return n, err
}

type uploadBody struct {
	io.ReadCloser
	controller *http.ResponseController
}

func (r *uploadBody) Read(p []byte) (int, error) {
	if err := r.controller.SetReadDeadline(time.Now().Add(uploadIdleTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return 0, err
	}
	return r.ReadCloser.Read(p)
}

func uploadErrorStatus(err error, fallback int) int {
	var tooLarge *http.MaxBytesError
	var network net.Error
	switch {
	case errors.Is(err, errUploadQuota), errors.As(err, &tooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.As(err, &network) && network.Timeout():
		return http.StatusRequestTimeout
	default:
		return fallback
	}
}
