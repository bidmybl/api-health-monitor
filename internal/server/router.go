package server

import (
	"net/http"

	endpointTransport "github.com/bidmybl/api-health-monitor/internal/features/endpoints/transport"
)

func NewRouter(endpointHandler *endpointTransport.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /endpoints", endpointHandler.CreateEndpoint)
	mux.HandleFunc("GET /endpoints/{id}", endpointHandler.GetEndpoint)
	mux.HandleFunc("GET /endpoints", endpointHandler.GetAllEndpoints)
	mux.HandleFunc("PATCH /endpoints/{id}", endpointHandler.UpdateEndpoint)
	mux.HandleFunc("DELETE /endpoints/{id}", endpointHandler.DeleteEndpoint)

	return mux
}
