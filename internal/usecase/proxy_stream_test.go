package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// --- mocks ---

type streamMockChatStreamer struct {
	events <-chan vo.StreamEvent
	err    error
	calls  int
}

func (m *streamMockChatStreamer) StreamChat(_ context.Context, _ vo.ProviderCredentials, _ *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	m.calls++
	return m.events, m.err
}

type streamMockAccountSelector struct {
	accounts []*entity.Account
	err      error
	callIdx  int
}

func (m *streamMockAccountSelector) Execute(_ context.Context, _ *vo.AccountID, _ *vo.AccountID, _ time.Time) (*entity.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.accounts) {
		return nil, errs.ErrAllAccountsExhausted
	}
	acc := m.accounts[m.callIdx]
	m.callIdx++
	return acc, nil
}

type streamMockFallbackHandler struct {
	results []FallbackResult
	errs    []error
	callIdx int
}

func (m *streamMockFallbackHandler) Execute(_ context.Context, _ *entity.Account, _ vo.ErrorClassification) (FallbackResult, error) {
	if m.callIdx >= len(m.results) {
		return FallbackResult{}, errors.New("unexpected fallback call")
	}
	r := m.results[m.callIdx]
	var e error
	if m.callIdx < len(m.errs) {
		e = m.errs[m.callIdx]
	}
	m.callIdx++
	return r, e
}

type streamMockAccountStatusWriter struct {
	recordSuccessCalls []vo.AccountID
	recordSuccessErr   error
}

func (m *streamMockAccountStatusWriter) UpdateStatus(_ context.Context, _ *entity.Account) error {
	return nil
}

func (m *streamMockAccountStatusWriter) RecordSuccess(_ context.Context, id vo.AccountID, _ time.Time) error {
	m.recordSuccessCalls = append(m.recordSuccessCalls, id)
	return m.recordSuccessErr
}

type streamMockSessionWriter struct {
	saved *entity.Session
	err   error
	calls int
}

func (m *streamMockSessionWriter) Save(_ context.Context, s *entity.Session) error {
	m.calls++
	m.saved = s
	return m.err
}

type streamMockSessionReader struct {
	session *entity.Session
	err     error
}

func (m *streamMockSessionReader) GetOrCreate(_ context.Context, _ vo.APIKeyID, _ vo.ModelName) (*entity.Session, error) {
	return m.session, m.err
}

// streamClassifiedErr implements ClassifiedError.
type streamClassifiedErr struct {
	msg            string
	classification vo.ErrorClassification
}

func (e *streamClassifiedErr) Error() string                          { return e.msg }
func (e *streamClassifiedErr) Classification() vo.ErrorClassification { return e.classification }

var streamNow = time.Date(2026, 3, 17, 15, 0, 0, 0, time.UTC)

func streamTestAccount(t *testing.T) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		"stream-account",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func streamTestSession(t *testing.T) *entity.Session {
	t.Helper()
	model, err := vo.ParseModelName("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("ParseModelName: %v", err)
	}
	sess, err := entity.NewSession(vo.NewSessionID(), vo.NewAPIKeyID(), model, 30*time.Minute, streamNow)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return sess
}

func streamTestRequest() *vo.ChatRequest {
	return &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
		MaxTokens: 1024,
	}
}

func makeStreamEvents(evts ...vo.StreamEvent) <-chan vo.StreamEvent {
	ch := make(chan vo.StreamEvent, len(evts))
	for _, e := range evts {
		ch <- e
	}
	close(ch)
	return ch
}

func drainStreamEvents(ch <-chan vo.StreamEvent) []vo.StreamEvent {
	var out []vo.StreamEvent
	for e := range ch {
		out = append(out, e)
	}
	return out
}

type streamSwitchingChatStreamer struct {
	streamers []ChatStreamer
	callIdx   int
}

func (s *streamSwitchingChatStreamer) StreamChat(ctx context.Context, creds vo.ProviderCredentials, req *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	if s.callIdx >= len(s.streamers) {
		return nil, errors.New("no more streamers")
	}
	st := s.streamers[s.callIdx]
	s.callIdx++
	return st.StreamChat(ctx, creds, req)
}

type streamAlwaysFailChatStreamer struct {
	err error
}

func (s *streamAlwaysFailChatStreamer) StreamChat(_ context.Context, _ vo.ProviderCredentials, _ *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	return nil, s.err
}

// --- helpers ---

type streamTestDeps struct {
	streamer  ChatStreamer
	selector  *streamMockAccountSelector
	fallback  *streamMockFallbackHandler
	writer    *streamMockAccountStatusWriter
	sessRead  *streamMockSessionReader
	sessWrite *streamMockSessionWriter
	clock     *fixedClock
}

func newStreamUC(d streamTestDeps) *ProxyStreamUseCase {
	return NewProxyStreamUseCase(d.streamer, d.selector, d.fallback, d.writer, d.sessRead, d.sessWrite, d.clock)
}

// --- tests ---

func TestProxyStream_Success(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	events := makeStreamEvents(
		vo.StreamEvent{Type: vo.EventDelta, Content: "Hello"},
		vo.StreamEvent{Type: vo.EventStop},
	)

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{events: events},
		selector:  &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := drainStreamEvents(result.Events)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Content != "Hello" {
		t.Errorf("event[0].Content = %q, want %q", got[0].Content, "Hello")
	}
	if got[1].Type != vo.EventStop {
		t.Errorf("event[1].Type = %q, want %q", got[1].Type, vo.EventStop)
	}
	if len(result.Retries) != 0 {
		t.Errorf("Retries = %d, want 0", len(result.Retries))
	}

	// Wait for goroutine cleanup
	time.Sleep(10 * time.Millisecond)
	if len(d.writer.recordSuccessCalls) != 1 {
		t.Errorf("RecordSuccess called %d times, want 1", len(d.writer.recordSuccessCalls))
	}
	if d.sessWrite.calls != 1 {
		t.Errorf("SessionWriter.Save called %d times, want 1", d.sessWrite.calls)
	}
}

func TestProxyStream_SessionReaderError(t *testing.T) {
	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{},
		selector:  &streamMockAccountSelector{},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{err: errors.New("session error")},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err == nil {
		t.Fatal("expected error when session reader fails")
	}
}

func TestProxyStream_AccountSelectorError(t *testing.T) {
	sess := streamTestSession(t)

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{},
		selector:  &streamMockAccountSelector{err: errs.ErrAllAccountsExhausted},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Fatalf("err = %v, want ErrAllAccountsExhausted", err)
	}
}

func TestProxyStream_PreStreamClassifiedError_NoFallback(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	classErr := &streamClassifiedErr{msg: "bad request", classification: vo.ErrClient}

	d := streamTestDeps{
		streamer: &streamMockChatStreamer{err: classErr},
		selector: &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback: &streamMockFallbackHandler{
			results: []FallbackResult{{ShouldFallback: false, Classification: vo.ErrClient}},
		},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err == nil {
		t.Fatal("expected error for client error without fallback")
	}
}

func TestProxyStream_PreStreamClassifiedError_WithFallback(t *testing.T) {
	acc1 := streamTestAccount(t)
	acc2 := streamTestAccount(t)
	sess := streamTestSession(t)

	classErr := &streamClassifiedErr{msg: "rate limited", classification: vo.ErrRateLimit}
	events := makeStreamEvents(
		vo.StreamEvent{Type: vo.EventDelta, Content: "Hi"},
		vo.StreamEvent{Type: vo.EventStop},
	)

	failStreamer := &streamMockChatStreamer{err: classErr}
	okStreamer := &streamMockChatStreamer{events: events}
	switchStreamer := &streamSwitchingChatStreamer{
		streamers: []ChatStreamer{failStreamer, okStreamer},
	}

	d := streamTestDeps{
		streamer: switchStreamer,
		selector: &streamMockAccountSelector{accounts: []*entity.Account{acc1, acc2}},
		fallback: &streamMockFallbackHandler{
			results: []FallbackResult{{ShouldFallback: true, Classification: vo.ErrRateLimit}},
		},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := drainStreamEvents(result.Events)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if len(result.Retries) != 1 {
		t.Errorf("Retries = %d, want 1", len(result.Retries))
	}
	if result.Retries[0].Classification != vo.ErrRateLimit {
		t.Errorf("Retries[0].Classification = %q, want %q", result.Retries[0].Classification, vo.ErrRateLimit)
	}
}

func TestProxyStream_PreStreamUnclassifiedError(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{err: errors.New("connection refused")},
		selector:  &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err == nil {
		t.Fatal("expected error for unclassified error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("err = %q, want %q", err.Error(), "connection refused")
	}
}

func TestProxyStream_FallbackHandlerError(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	classErr := &streamClassifiedErr{msg: "rate limited", classification: vo.ErrRateLimit}

	d := streamTestDeps{
		streamer: &streamMockChatStreamer{err: classErr},
		selector: &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback: &streamMockFallbackHandler{
			results: []FallbackResult{{}},
			errs:    []error{errors.New("fallback db error")},
		},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err == nil {
		t.Fatal("expected error when fallback handler fails")
	}
}

func TestProxyStream_MaxRetriesExhausted(t *testing.T) {
	sess := streamTestSession(t)

	accounts := make([]*entity.Account, 11)
	for i := range accounts {
		accounts[i] = streamTestAccount(t)
	}

	classErr := &streamClassifiedErr{msg: "rate limited", classification: vo.ErrRateLimit}
	results := make([]FallbackResult, 11)
	for i := range results {
		results[i] = FallbackResult{ShouldFallback: true, Classification: vo.ErrRateLimit}
	}

	d := streamTestDeps{
		streamer:  &streamAlwaysFailChatStreamer{err: classErr},
		selector:  &streamMockAccountSelector{accounts: accounts},
		fallback:  &streamMockFallbackHandler{results: results},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Fatalf("err = %v, want ErrAllAccountsExhausted", err)
	}
}

func TestProxyStream_GoroutineRecordSuccessError(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	events := makeStreamEvents(vo.StreamEvent{Type: vo.EventStop})

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{events: events},
		selector:  &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{recordSuccessErr: errors.New("db error")},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Drain — goroutine should not panic even though RecordSuccess fails
	drainStreamEvents(result.Events)
	time.Sleep(10 * time.Millisecond)
}

func TestProxyStream_GoroutineSessionWriterError(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	events := makeStreamEvents(vo.StreamEvent{Type: vo.EventStop})

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{events: events},
		selector:  &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{err: errors.New("session save error")},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Drain — goroutine should not panic even though Save fails
	drainStreamEvents(result.Events)
	time.Sleep(10 * time.Millisecond)
}

func TestProxyStream_EventErrorPassedThrough(t *testing.T) {
	acc := streamTestAccount(t)
	sess := streamTestSession(t)

	errMsg := "provider error mid-stream"
	events := makeStreamEvents(
		vo.StreamEvent{Type: vo.EventDelta, Content: "partial"},
		vo.StreamEvent{Type: vo.EventError, Error: &errMsg},
	)

	d := streamTestDeps{
		streamer:  &streamMockChatStreamer{events: events},
		selector:  &streamMockAccountSelector{accounts: []*entity.Account{acc}},
		fallback:  &streamMockFallbackHandler{},
		writer:    &streamMockAccountStatusWriter{},
		sessRead:  &streamMockSessionReader{session: sess},
		sessWrite: &streamMockSessionWriter{},
		clock:     &fixedClock{now: streamNow},
	}
	uc := newStreamUC(d)

	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), streamTestRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := drainStreamEvents(result.Events)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[1].Type != vo.EventError {
		t.Errorf("event[1].Type = %q, want %q", got[1].Type, vo.EventError)
	}
	if *got[1].Error != errMsg {
		t.Errorf("event[1].Error = %q, want %q", *got[1].Error, errMsg)
	}
}
