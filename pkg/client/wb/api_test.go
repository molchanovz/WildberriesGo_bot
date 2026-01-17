package wb

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
	"tradebot/pkg/db"
	"tradebot/pkg/db/test"
)

func TestReviews(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)
	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: test.Ptr("WB")})
	if err != nil {
		return
	} else if cabinet == nil {
		return
	}

	Convey("Should get google info", t, func() {
		client := NewClient(cabinet.Key)
		got, err := client.Reviews()
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
	})
}

func TestOrders(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)
	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: test.Ptr("WB")})
	if err != nil {
		return
	} else if cabinet == nil {
		return
	}

	Convey("Should get google info", t, func() {
		client := NewClient(cabinet.Key)
		got, err := client.GetAllOrders(1, 1)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
	})
}

func TestClient_AnswerReview(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)
	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: test.Ptr("WB")})
	if err != nil {
		return
	} else if cabinet == nil {
		return
	}

	Convey("Should get google info", t, func() {
		client := NewClient(cabinet.Key)
		err = client.AnswerReview("wP6Ju7VETs5MoHnitTqj", "Благодарим Вас за высокую оценку и отзыв!")
		if err != nil {
			fmt.Println(err.Error())
		}
	})
}

func TestClient_getOrdersBySupplyID(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)
	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: test.Ptr("WB")})
	if err != nil {
		return
	} else if cabinet == nil {
		return
	}

	Convey("Should get google info", t, func() {
		client := NewClient(cabinet.Key)
		got, err := client.getOrdersBySupplyID("WB-GI-204196591")
		So(err, ShouldBeNil)
		t.Log(got)
	})
}

func TestClient_GetOrdersFBS(t *testing.T) {
	dbo := test.Setup(t)
	repo := db.NewTradebotRepo(dbo)
	cabinet, err := repo.OneCabinet(t.Context(), &db.CabinetSearch{Marketplace: test.Ptr("WB")})
	if err != nil {
		return
	} else if cabinet == nil {
		return
	}

	dateFrom := int(GetUnix(time.Now().AddDate(0, 0, -(7 + 1))))
	dateTo := int(GetUnix(time.Now().AddDate(0, 0, -7)))

	Convey("", t, func() {
		client := NewClient(cabinet.Key)
		got, err := client.GetOrdersFBS(dateFrom, dateTo)
		So(err, ShouldBeNil)
		for _, order := range got.OrdersFBS {
			t.Log(order.ID)
		}
	})
}
