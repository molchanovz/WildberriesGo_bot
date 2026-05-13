package wb

import (
	"github.com/BurntSushi/toml"
	"github.com/openai/openai-go/v3"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
	"tradebot/pkg/tradeplus/test"
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
	reviews, err := m.Reviews(t.Context())
	require.NoError(t, err)
	t.Log(reviews)
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
