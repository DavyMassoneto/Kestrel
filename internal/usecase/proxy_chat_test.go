package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
)

// --- mocks ---

type stubChatSender struct {
	responses []*vo.ChatResponse
	errs      []error
	callIdx   int
}

func (s *stubChatSender) SendChat(_ context.Context, _ vo.ProviderCredentials, _ *vo.ChatRequest) (*vo.ChatResponse, error) {
	i := s.callIdx
	s.callIdx++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	if i < len(s.responses) {
		return s.responses[i], nil
	}
	return &vo.ChatResponse{ID: "default"}, nil
}

type stubAccountSelector struct {
	accounts []*entity.Account
	errs     []error
	callIdx  int
}

func (s *stubAccountSelector) Execute(_ context.Context, _ *vo.AccountID, _ *vo.AccountID, _ time.Time) (*entity.Account, error) {
	i := s.callIdx
	s.callIdx++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	if i < len(s.accounts) {
		return s.accounts[i], nil
	}
	return nil, errs.ErrAllAccountsExhausted
}

type stubFallbackHandler struct {
	results []usecase.FallbackResult
	errs    []error
	callIdx int
}

func (s *stubFallbackHandler) Execute(_ context.Context, _ *entity.Account, _ vo.ErrorClassification) (usecase.FallbackResult, error) {
	i := s.callIdx
	s.callIdx++
	if i < len(s.errs) && s.errs[i] != nil {
		return usecase.FallbackResult{}, s.errs[i]
	}
	if i < len(s.results) {
		return s.results[i], nil
	}
	return usecase.FallbackResult{}, nil
}

type stubSessionReader struct {
	session *entity.Session
	err     error
}

func (s *stubSessionReader) GetOrCreate(_ context.Context, _ vo.APIKeyID, _ vo.ModelName) (*entity.Session, error) {
	return s.session, s.err
}

type stubSessionWriter struct {
	saved   *entity.Session
	err     error
	callCnt int
}

func (s *stubSessionWriter) Save(_ context.Context, sess *entity.Session) error {
	s.saved = sess
	s.callCnt++
	return s.err
}

type stubAccountWriter struct {
	callCnt int
	err     error
}

func (s *stubAccountWriter) UpdateStatus(_ context.Context, _ *entity.Account) error {
	s.callCnt++
	return s.err
}

func (s *stubAccountWriter) RecordSuccess(_ context.Context, _ vo.AccountID, _ time.Time) error {
	return nil
}

type proxyClock struct {
	now time.Time
}

func (c *proxyClock) Now() time.Time { return c.now }

// --- classifiedError for tests ---

type testClassifiedError struct {
	msg            string
	classification vo.ErrorClassification
}

func (e *testClassifiedError) Error() string                        { return e.msg }
func (e *testClassifiedError) Classification() vo.ErrorClassification { return e.classification }

// --- helpers ---

func newTestSession(t *testing.T) *entity.Session {
	t.Helper()
	m, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	sess, err := entity.NewSession(vo.NewSessionID(), vo.NewAPIKeyID(), m, 30*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return sess
}

func newTestAccount(t *testing.T, name string) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(vo.NewAccountID(), name, vo.NewSensitiveString("sk-test"), "https://api.anthropic.com", 0)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func testChatReq() *vo.ChatRequest {
	return &vo.ChatRequest{
		Model:     vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages:  []vo.Message{{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 1024,
	}
}

// --- tests ---

func TestProxyChat_SuccessFirstAttempt(t *testing.T) {
	acc := newTestAccount(t, "acc1")
	resp := &vo.ChatResponse{ID: "msg_1", Content: "Hello", StopReason: "end_turn"}

	sender := &stubChatSender{responses: []*vo.ChatResponse{resp}}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc}}
	fallback := &stubFallbackHandler{}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	sessWriter := &stubSessionWriter{}
	accWriter := &stubAccountWriter{}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, sessWriter, accWriter, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == nil {
		t.Fatal("response should not be nil")
	}
	if result.Response.ID != "msg_1" {
		t.Errorf("ID = %q; want msg_1", result.Response.ID)
	}
	if len(result.Retries) != 0 {
		t.Errorf("Retries = %d; want 0", len(result.Retries))
	}
	if accWriter.callCnt != 1 {
		t.Errorf("UpdateStatus calls = %d; want 1", accWriter.callCnt)
	}
	if sessWriter.callCnt != 1 {
		t.Errorf("SessionWriter.Save calls = %d; want 1", sessWriter.callCnt)
	}
}

func TestProxyChat_FallbackThenSuccess(t *testing.T) {
	acc1 := newTestAccount(t, "acc1")
	acc2 := newTestAccount(t, "acc2")
	resp := &vo.ChatResponse{ID: "msg_2", Content: "Ok"}

	classErr := &testClassifiedError{msg: "rate limited", classification: vo.ErrRateLimit}
	sender := &stubChatSender{
		responses: []*vo.ChatResponse{nil, resp},
		errs:      []error{classErr, nil},
	}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc1, acc2}}
	fallback := &stubFallbackHandler{results: []usecase.FallbackResult{
		{ShouldFallback: true, Classification: vo.ErrRateLimit},
	}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	sessWriter := &stubSessionWriter{}
	accWriter := &stubAccountWriter{}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, sessWriter, accWriter, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == nil || result.Response.ID != "msg_2" {
		t.Error("expected response from second account")
	}
	if len(result.Retries) != 1 {
		t.Errorf("Retries = %d; want 1", len(result.Retries))
	}
	if result.Retries[0].Classification != vo.ErrRateLimit {
		t.Errorf("retry classification = %v; want rate_limit", result.Retries[0].Classification)
	}
}

func TestProxyChat_MultipleRetries(t *testing.T) {
	acc1 := newTestAccount(t, "acc1")
	acc2 := newTestAccount(t, "acc2")
	acc3 := newTestAccount(t, "acc3")
	resp := &vo.ChatResponse{ID: "msg_3"}

	classErr := &testClassifiedError{msg: "overloaded", classification: vo.ErrOverloaded}
	sender := &stubChatSender{
		responses: []*vo.ChatResponse{nil, nil, resp},
		errs:      []error{classErr, classErr, nil},
	}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc1, acc2, acc3}}
	fallback := &stubFallbackHandler{results: []usecase.FallbackResult{
		{ShouldFallback: true}, {ShouldFallback: true},
	}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	sessWriter := &stubSessionWriter{}
	accWriter := &stubAccountWriter{}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, sessWriter, accWriter, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Retries) != 2 {
		t.Errorf("Retries = %d; want 2", len(result.Retries))
	}
}

func TestProxyChat_AllAccountsExhausted(t *testing.T) {
	selector := &stubAccountSelector{errs: []error{errs.ErrAllAccountsExhausted}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(nil, selector, nil, sessReader, &stubSessionWriter{}, &stubAccountWriter{}, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Errorf("err = %v; want ErrAllAccountsExhausted", err)
	}
	if result.Response != nil {
		t.Error("response should be nil")
	}
}

func TestProxyChat_UnclassifiedError(t *testing.T) {
	acc := newTestAccount(t, "acc1")
	plainErr := errors.New("connection timeout")

	sender := &stubChatSender{errs: []error{plainErr}}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, nil, sessReader, &stubSessionWriter{}, &stubAccountWriter{}, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection timeout" {
		t.Errorf("err = %q; want 'connection timeout'", err.Error())
	}
	if len(result.Retries) != 0 {
		t.Errorf("Retries = %d; want 0", len(result.Retries))
	}
}

func TestProxyChat_NoFallback(t *testing.T) {
	acc := newTestAccount(t, "acc1")
	classErr := &testClassifiedError{msg: "bad request", classification: vo.ErrClient}

	sender := &stubChatSender{errs: []error{classErr}}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc}}
	fallback := &stubFallbackHandler{results: []usecase.FallbackResult{
		{ShouldFallback: false, Classification: vo.ErrClient},
	}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, &stubSessionWriter{}, &stubAccountWriter{}, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err == nil {
		t.Fatal("expected error")
	}
	if len(result.Retries) != 1 {
		t.Errorf("Retries = %d; want 1", len(result.Retries))
	}
}

func TestProxyChat_SafetyCapReached(t *testing.T) {
	acc := newTestAccount(t, "acc1")
	classErr := &testClassifiedError{msg: "overloaded", classification: vo.ErrOverloaded}

	// Create 10 accounts and 10 errors — all with ShouldFallback=true
	accounts := make([]*entity.Account, 10)
	sendErrs := make([]error, 10)
	fallbackResults := make([]usecase.FallbackResult, 10)
	for i := 0; i < 10; i++ {
		accounts[i] = acc
		sendErrs[i] = classErr
		fallbackResults[i] = usecase.FallbackResult{ShouldFallback: true}
	}

	sender := &stubChatSender{errs: sendErrs}
	selector := &stubAccountSelector{accounts: accounts}
	fallback := &stubFallbackHandler{results: fallbackResults}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, &stubSessionWriter{}, &stubAccountWriter{}, clock)
	result, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Errorf("err = %v; want ErrAllAccountsExhausted", err)
	}
	if len(result.Retries) != 10 {
		t.Errorf("Retries = %d; want 10", len(result.Retries))
	}
}

func TestProxyChat_SessionReaderError(t *testing.T) {
	sessReader := &stubSessionReader{err: errors.New("session store broken")}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(nil, nil, nil, sessReader, nil, nil, clock)
	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProxyChat_FallbackHandlerError(t *testing.T) {
	acc := newTestAccount(t, "acc1")
	classErr := &testClassifiedError{msg: "rate limited", classification: vo.ErrRateLimit}

	sender := &stubChatSender{errs: []error{classErr}}
	selector := &stubAccountSelector{accounts: []*entity.Account{acc}}
	fallback := &stubFallbackHandler{errs: []error{errors.New("fallback db error")}}
	sessReader := &stubSessionReader{session: newTestSession(t)}
	clock := &proxyClock{now: time.Now()}

	uc := usecase.NewProxyChatUseCase(sender, selector, fallback, sessReader, &stubSessionWriter{}, &stubAccountWriter{}, clock)
	_, err := uc.Execute(context.Background(), vo.NewAPIKeyID(), testChatReq())

	if err == nil {
		t.Fatal("expected error")
	}
}
