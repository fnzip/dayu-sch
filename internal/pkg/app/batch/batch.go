package batch

import (
	"context"
	"dayusch/internal/pkg/api/cfbatch"
	"dayusch/internal/pkg/db"
	"dayusch/internal/pkg/repo"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	cfBatchUrl := os.Getenv("CF_BATCH_URL")
	cfBatchToken := os.Getenv("CF_BATCH_TOKEN")
	wgPrivateKey := os.Getenv("WG_PRIVATEKEY")
	wgEndpoint := os.Getenv("WG_ENDPOINT")

	md, err := db.NewDbCon(a.ctx, uri, dbName)
	if err != nil {
		log.Fatal(err)
	}

	ar := repo.NewAppRepo(md)
	ur := repo.NewUserRepo(md)

	cfb := cfbatch.NewCFBatchApi(cfBatchUrl, cfBatchToken)

	endpoint, err := resolveIPPAndPort(wgEndpoint)
	if err != nil {
		log.Fatal(err)
	}

	index := primitive.NilObjectID

	for {
		select {
		case <-a.ctx.Done():
			log.Info("context cancelled, exiting loop")
			return
		default:
		}

		dialer, err := NewWGDialer(wgPrivateKey, endpoint)
		if err != nil {
			log.Fatal(err)
		}

		cfb.SetDialContext(dialer.WireDialer.tnet.DialContext)

		apps, err := ar.GetClaimAppCodes(a.ctx)
		if err != nil {
			log.Fatal(err)
		}

		users, err := ur.GetClaimUsers(a.ctx, apps, 120, index)
		if err != nil {
			log.Fatal(err)
		}

		log.Info("got users", "total", len(users))

		if len(users) == 0 {
			index = primitive.NilObjectID
			continue
		}

		index = users[len(users)-1].ID

		// Split users into chunks of 25
		userChunks := make([][]*repo.ModelUser, 0)
		chunkSize := 10

		for i := 0; i < len(users); i += chunkSize {
			end := i + chunkSize
			if end > len(users) {
				end = len(users)
			}
			userChunks = append(userChunks, users[i:end])
		}

		log.Info("split into", "total", len(userChunks), "chunk", chunkSize)

		// Use semaphore to limit concurrent goroutines
		sem := make(chan struct{}, 10) // Limit to 10 concurrent goroutines
		var wg sync.WaitGroup

		for _, chunk := range userChunks {
			wg.Add(1)

			go func(userChunk []*repo.ModelUser) {
				defer wg.Done()
				sem <- struct{}{}        // Acquire semaphore
				defer func() { <-sem }() // Release semaphore

				// Convert chunk to CFBatchUser
				var usersToClaim []cfbatch.CFBatchUser
				for _, user := range userChunk {
					userClaim := cfbatch.CFBatchUser{
						ID:                 user.ID.Hex(),
						AppCode:            user.AppCode,
						Username:           user.Username,
						Balance:            user.Balance,
						Coin:               user.Coin,
						WalletURL:          user.AppDetail.WalletURL,
						WalletCustomInfoID: user.AppDetail.WalletCustomInfoID,
						VipActivityId:      user.AppDetail.VipActivityId,
					}
					usersToClaim = append(usersToClaim, userClaim)
				}

				cfb.SendBatch(a.ctx, usersToClaim)
			}(chunk)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		dialer.WireDialer.Device.Close()
	}
}
