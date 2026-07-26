package httpapi

import "net/http"

func (handler server) CreateEvents(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
