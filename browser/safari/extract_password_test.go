package safari

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		server   string
		port     uint32
		path     string
		want     string
	}{
		{
			name:     "https default port",
			protocol: "htps",
			server:   "github.com",
			port:     443,
			want:     "https://github.com",
		},
		{
			name:     "https custom port",
			protocol: "htps",
			server:   "example.com",
			port:     8443,
			want:     "https://example.com:8443",
		},
		{
			name:     "http with path",
			protocol: "http",
			server:   "192.168.1.1",
			port:     80,
			path:     "/admin",
			want:     "http://192.168.1.1/admin",
		},
		{
			name:     "http non-default port",
			protocol: "http",
			server:   "localhost",
			port:     8080,
			want:     "http://localhost:8080",
		},
		{
			name:     "empty server returns empty",
			protocol: "htps",
			server:   "",
			port:     443,
			want:     "",
		},
		{
			name:     "empty protocol defaults to https",
			protocol: "",
			server:   "example.com",
			port:     0,
			want:     "https://example.com",
		},
		{
			name:     "smb protocol",
			protocol: "smb ",
			server:   "fileserver",
			port:     445,
			want:     "smb://fileserver:445",
		},
		{
			name:     "ftp default port",
			protocol: "ftp ",
			server:   "ftp.example.com",
			port:     21,
			want:     "ftp://ftp.example.com",
		},
		{
			name:     "root path ignored",
			protocol: "htps",
			server:   "example.com",
			port:     443,
			path:     "/",
			want:     "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildURL(tt.protocol, tt.server, tt.port, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
