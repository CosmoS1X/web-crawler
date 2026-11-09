package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/CosmoS1X/web-crawler/crawler"
	"github.com/CosmoS1X/web-crawler/internal/app"
)

const (
	commandName = "web-crawler"
	testVersion = "1.2.3"
)

func newTestCommand(client *http.Client) (*cli.Command, *bytes.Buffer) {
	cmd := app.New(client, testVersion)
	buf := &bytes.Buffer{}
	cmd.Writer = buf
	cmd.ErrWriter = buf
	return cmd, buf
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	cmd, buf := newTestCommand(&http.Client{})

	require.NoError(t, cmd.Run(t.Context(), []string{commandName, "--version"}))
	assert.Equal(t, commandName+" version "+testVersion+"\n", buf.String())
}

func TestHelpDescribesFlags(t *testing.T) {
	t.Parallel()

	cmd, buf := newTestCommand(&http.Client{})
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	require.NoError(t, cmd.Run(t.Context(), []string{commandName, "--help"}))

	out := buf.String()
	for _, want := range []string{
		"crawl depth",
		"number of retries for failed requests",
		"delay between requests (example: 200ms, 1s)",
		"per-request timeout",
		"limit requests per second (overrides delay)",
		"custom user agent",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRunPrintsJSONReport(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cmd, buf := newTestCommand(srv.Client())
	require.NoError(t, cmd.Run(t.Context(), []string{commandName, srv.URL}))

	var rep crawler.Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rep), "output: %s", buf.String())
	assert.Equal(t, srv.URL, rep.RootURL)
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, http.StatusOK, rep.Pages[0].HTTPStatus)

	out := buf.String()
	require.NotEmpty(t, out)
	assert.Equal(t, "{", out[:1], "output must not contain text before the JSON object")
	assert.True(t, strings.HasSuffix(out, "}\n"), "output must end with a single trailing newline")
	assert.False(t, strings.HasSuffix(out, "\n\n"), "output must not contain extra trailing newlines")
	assert.True(t, json.Valid([]byte(strings.TrimSuffix(out, "\n"))))
}

func TestRunUsesRetriesFlag(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cmd, buf := newTestCommand(srv.Client())
	require.NoError(t, cmd.Run(t.Context(), []string{
		commandName, "--retries", "2", srv.URL,
	}))

	var rep crawler.Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rep), "output: %s", buf.String())
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, http.StatusOK, rep.Pages[0].HTTPStatus)
	assert.Equal(t, "ok", rep.Pages[0].Status)
	assert.Equal(t, int32(3), calls.Load())
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection failed")
}

func TestRunNetworkErrorExitsSuccessfully(t *testing.T) {
	t.Parallel()

	cmd, buf := newTestCommand(&http.Client{Transport: failingTransport{}})
	require.NoError(t, cmd.Run(t.Context(), []string{commandName, "https://example.com"}))

	var rep crawler.Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rep), "output: %s", buf.String())
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, "error", rep.Pages[0].Status)
}

func TestRunRequiresURL(t *testing.T) {
	t.Parallel()

	cmd, _ := newTestCommand(&http.Client{})
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	require.Error(t, cmd.Run(t.Context(), []string{commandName}))
}

func TestRunAnalyzeError(t *testing.T) {
	t.Parallel()

	cmd, _ := newTestCommand(&http.Client{})
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := cmd.Run(t.Context(), []string{commandName, "ftp://example.com"})
	require.ErrorIs(t, err, crawler.ErrInvalidURL)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunWriteError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cmd, _ := newTestCommand(srv.Client())
	cmd.Writer = failingWriter{}
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := cmd.Run(t.Context(), []string{commandName, srv.URL})
	require.ErrorContains(t, err, "write report")
}
