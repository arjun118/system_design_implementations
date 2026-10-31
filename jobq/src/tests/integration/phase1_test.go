//go:build integration

package integration_test

import (
	"testing"

	"arjun118.github.io/jobq/internal/httpapi"
)

func newTestServer() *httpapi.Server {
	return httpapi.NewServer()
}

func TestPhase1(t *testing.T) {
	server := newTestServer()

}
