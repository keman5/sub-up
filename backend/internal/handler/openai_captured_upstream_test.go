package handler

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Shared by default and unit-tagged handler regressions.
// astraProSuccessResponse echoes reasoning.mode=pro without downgrading it.
const astraProSuccessResponse = `{"id":"resp_astra","model":"gpt-6-astra","status":"completed",` +
	`"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],` +
	`"usage":{"input_tokens":1,"output_tokens":2},` +
	`"reasoning":{"mode":"pro","effort":"max"}}`

// astraProCapturedUpstream records URL/accountID/body per forwarded attempt.
type astraProCapturedUpstream struct {
	service.HTTPUpstream
	mu         sync.Mutex
	urls       []string
	accountIDs []int64
	bodies     [][]byte
	answers    []*http.Response
}

func newAstraProCapturedUpstream(answers ...*http.Response) *astraProCapturedUpstream {
	return &astraProCapturedUpstream{answers: answers}
}

func (u *astraProCapturedUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	body, _ := io.ReadAll(req.Body)
	u.urls = append(u.urls, req.URL.String())
	u.accountIDs = append(u.accountIDs, accountID)
	u.bodies = append(u.bodies, body)
	var resp *http.Response
	if len(u.answers) > 0 {
		resp = u.answers[0]
		u.answers = u.answers[1:]
	}
	if resp == nil {
		resp = &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil))}
	}
	return resp, nil
}

func (u *astraProCapturedUpstream) snapshot() ([]string, []int64, [][]byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.urls...), append([]int64(nil), u.accountIDs...), append([][]byte(nil), u.bodies...)
}

func astra200() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(astraProSuccessResponse)),
	}
}
