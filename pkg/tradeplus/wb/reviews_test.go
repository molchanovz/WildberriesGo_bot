package wb

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
	"tradebot/pkg/tradeplus/test"

	"github.com/BurntSushi/toml"
	"github.com/openai/openai-go/v3"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
)

var (
	cfg    = test.Cfg
	gptSrv *chatgptsrv.Client
)

func TestMain(m *testing.M) {
	if _, err := toml.DecodeFile("/Users/sergey/GolandProjects/tradebot/cfg/local.toml", &cfg); err != nil {
		return
	}
	gptSrv = chatgptsrv.NewClient(cfg.Service.ChatGPTSrvURL, &http.Client{Timeout: time.Second * 30})
	m.Run()
}

func TestReviewManager_Reviews(t *testing.T) {
	dbc, err := test.Setup()
	repo := db.NewTradebotRepo(dbc)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: openai.Ptr("WB")})
	require.NoError(t, err)

	m := NewReviewManager(*dbc, tradeplus.NewCabinet(cabinet), gptSrv)

	require.NoError(t, m.Fetch(t.Context()))

	sent := 0
	err = m.ProcessPending(t.Context(), func(_ context.Context, r tradeplus.Review) error {
		sent++
		t.Logf("to operator: %s -> %s", r.ExternalID, r.Answer)
		return nil
	})
	require.NoError(t, err)
	t.Logf("sent to operator: %d", sent)
}

func TestReviewManager_AnswerReview(t *testing.T) {
	dbc, err := test.Setup()
	repo := db.NewTradebotRepo(dbc)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: openai.Ptr("WB")})
	require.NoError(t, err)

	m := NewReviewManager(*dbc, tradeplus.NewCabinet(cabinet), gptSrv)

	Convey("success answer", t, func() {
		err = m.AnswerReview(t.Context(), "ftey4CV8ccvlbmQ5Acjh")
		So(err, ShouldBeNil)
	})
}

// TestSetAnswerEmptyPositive verifies that a text-less 4–5★ review is answered
// with a canned thank-you plus the recommendation and routed for auto-posting,
// WITHOUT calling the model. The nil chatgpt client here is the guard: if the
// short-circuit failed and SetAnswer reached the model, it would panic.
func TestSetAnswerEmptyPositive(t *testing.T) {
	m := ReviewManager{} // zero value: chatgpt is nil on purpose

	for _, stars := range []int{4, 5} {
		nr := &tradeplus.Review{Review: db.Review{Valuation: stars}}
		require.True(t, nr.IsEmpty(), "review with %d★ and no content must be empty", stars)

		err := m.SetAnswer(context.Background(), nr, tradeplus.Product{})
		require.NoError(t, err, "%d★", stars)
		require.False(t, nr.ToOperator, "%d★: should auto-post, not go to operator", stars)
		require.NotEmpty(t, nr.Answer, "%d★", stars)
		require.Contains(t, nr.Answer, "Рекомендуем", "%d★: answer must carry a recommendation", stars)

		matched := false
		for _, canned := range emptyPositiveAnswers {
			if strings.HasPrefix(nr.Answer, canned) {
				matched = true
				break
			}
		}
		require.True(t, matched, "%d★: answer must start with a canned thank-you: %q", stars, nr.Answer)
	}
}
