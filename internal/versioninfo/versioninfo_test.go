package versioninfo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CosmoS1X/web-crawler/internal/versioninfo"
)

const (
	devVersion      = "dev"
	releasedVersion = "v1.0.0"
	nextVersion     = "v1.2.3"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		injected      string
		moduleVersion string
		want          string
	}{
		{
			name:          "injected version wins",
			injected:      nextVersion,
			moduleVersion: releasedVersion,
			want:          nextVersion,
		},
		{
			name:          "module version for go install",
			injected:      devVersion,
			moduleVersion: releasedVersion,
			want:          releasedVersion,
		},
		{
			name:          "empty injected version uses module version",
			injected:      "",
			moduleVersion: releasedVersion,
			want:          releasedVersion,
		},
		{
			name:          "development build",
			injected:      devVersion,
			moduleVersion: "(devel)",
			want:          devVersion,
		},
		{
			name:          "empty module version",
			injected:      devVersion,
			moduleVersion: "",
			want:          devVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, versioninfo.Resolve(tt.injected, tt.moduleVersion))
		})
	}
}

func TestModuleVersion(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, versioninfo.ModuleVersion())
}
