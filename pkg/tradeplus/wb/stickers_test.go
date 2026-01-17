package wb

import (
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
	"testing"
	"tradebot/pkg/db"
	"tradebot/pkg/db/test"
	"tradebot/pkg/tradeplus"
)

func TestStickerManager_orders(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)

	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: tradeplus.Ptr("WB")})
	require.NoError(t, err)

	m := NewStickerManager(cabinet.Key)

	Convey("", t, func() {
		got, err := m.orders("WB-GI-200509782")
		So(err, ShouldBeNil)
		for _, order := range got {
			t.Log(order.ID)
		}
	})
}
