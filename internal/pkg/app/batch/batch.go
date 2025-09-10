package batch

import (
	"context"
	"dayusch/internal/pkg/db"
	"dayusch/internal/pkg/repo"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

type BatchApp struct {
	ctx context.Context
}

func NewBatchApp(ctx context.Context) *BatchApp {
	return &BatchApp{
		ctx: ctx,
	}
}

func (a *BatchApp) Run() {
	uri := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	md, err := db.NewDbCon(ctx, uri, dbName)
	if err != nil {
		log.Fatal(err)
	}

	ar := repo.NewAppRepo(md)
	ur := repo.NewUserRepo(md)

	apps, err := ar.GetClaimAppCodes(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, app := range apps {
		log.Info("app detail", "app_code", app.AppCode)
	}

	users, err := ur.GetClaimUsers(ctx, apps, 5)
	if err != nil {
		log.Fatal(err)
	}

	for _, user := range users {
		log.Info("user detail", "app_code", user.AppCode, "username", user.Username, "url", user.AppDetail.WalletURL)
	}
}
