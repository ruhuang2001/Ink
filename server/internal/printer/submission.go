package printer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	memobirdapi "github.com/ruhuang2001/memobird-go/memobird"
)

// The SDK returns ordinary errors for rejection, transport and decoding
// failures. Observe only the final print POST to distinguish a complete
// rejection from a request whose external effect cannot be established.
type submissionTransport struct {
	transport http.RoundTripper
	attempted bool
	rejected  bool
}

func (s *submissionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, "/home/printpaper") {
		return s.transport.RoundTrip(request)
	}
	if err := request.Context().Err(); err != nil {
		return nil, err
	}
	s.attempted = true
	s.rejected = false
	response, err := s.transport.RoundTrip(request)
	if err != nil {
		return response, err
	}
	response.Body = &submissionResponseBody{ReadCloser: response.Body, outcome: s, status: response.StatusCode}
	return response, nil
}

const maxSubmissionResponseBytes = 64 << 10

type submissionResponseBody struct {
	io.ReadCloser
	outcome  *submissionTransport
	status   int
	body     []byte
	overflow bool
}

func (b *submissionResponseBody) Read(buffer []byte) (int, error) {
	n, err := b.ReadCloser.Read(buffer)
	if len(b.body)+n <= maxSubmissionResponseBytes {
		b.body = append(b.body, buffer[:n]...)
	} else {
		b.overflow = true
	}
	if errors.Is(err, io.EOF) && !b.overflow && b.status == http.StatusOK {
		var decoded struct {
			Code *int `json:"showapi_res_code"`
		}
		var response memobirdapi.PrintResponse
		if json.Unmarshal(b.body, &decoded) == nil && decoded.Code != nil &&
			json.Unmarshal(b.body, &response) == nil && !response.IsSuccess() {
			b.outcome.rejected = true
		}
	}
	return n, err
}

func (s *Service) newSubmissionClient(binding Binding) (*memobirdapi.Client, *submissionTransport, error) {
	outcome := &submissionTransport{transport: http.DefaultTransport}
	timeout := s.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client, err := memobirdapi.NewClient(memobirdapi.Config{
		AccessKey: s.accessKey, DeviceID: binding.DeviceIdentifier, UserID: binding.ProviderUserID, BaseURL: s.baseURL,
		HTTPClient: &http.Client{Timeout: timeout, Transport: outcome},
	})
	return client, outcome, err
}
