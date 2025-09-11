package schstat

import (
	"context"
	"dayusch/internal/pkg/db"
	"dayusch/internal/pkg/repo"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

type SchStat struct {
	ctx context.Context
}

func NewSchStat(ctx context.Context) *SchStat {
	return &SchStat{
		ctx: ctx,
	}
}

func (a *SchStat) Run() {
	uri := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")

	md, err := db.NewDbCon(a.ctx, uri, dbName)
	if err != nil {
		log.Fatal(err)
	}

	ar := repo.NewAppRepo(md)

	for {
		log.Info("Calling aggregation...")
		start := time.Now()
		err = ar.AggregateAppStats(a.ctx)
		duration := time.Since(start)
		if err != nil {
			log.Fatal(err)
		}

		log.Info("Aggregation completed", "duration", duration, "waiting", "15s")
		time.Sleep(15 * time.Second)
	}
}
