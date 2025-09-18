package dupdel

import (
	"context"
	"dayusch/internal/pkg/db"
	"dayusch/internal/pkg/repo"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DupDel struct {
	ctx context.Context
}

func NewDupDel(ctx context.Context) *DupDel {
	return &DupDel{
		ctx: ctx,
	}
}

func (d *DupDel) Run() {
	uri := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")

	md, err := db.NewDbCon(d.ctx, uri, dbName)
	if err != nil {
		log.Fatal(err)
	}

	ur := repo.NewUserRepo(md)

	// Initialize with nil ObjectID (000000000000000000000000)
	currentIndex := primitive.NilObjectID
	totalDeleted := 0

	log.Info("Starting duplicate deletion process...")

	for {
		select {
		case <-d.ctx.Done():
			log.Info("Context cancelled, stopping duplicate deletion", "total_deleted", totalDeleted)
			return
		default:
			// Find first user starting from current index
			user, err := ur.GetFirstUserFromIndex(d.ctx, currentIndex)
			if err != nil {
				log.Error("Error getting first user", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if user == nil {
				log.Info("No more users found, duplicate deletion completed", "total_deleted", totalDeleted)
				return
			}

			log.Info("Processing user", "id", user.ID.Hex(), "app_code", user.AppCode, "username", user.Username)

			// Find duplicates for this user
			duplicateIDs, err := ur.FindDuplicate(d.ctx, primitive.NilObjectID, user.AppCode, user.Username, user.ID)
			if err != nil {
				log.Error("Error finding duplicates", "error", err, "user_id", user.ID.Hex())
				currentIndex = user.ID
				continue
			}

			// Log duplicate count even if it's 0
			log.Info("Found duplicates", "count", len(duplicateIDs), "user_id", user.ID.Hex(), "app_code", user.AppCode, "username", user.Username)

			if len(duplicateIDs) > 0 {
				// Delete bulk duplicates
				err = ur.DeleteBulk(d.ctx, duplicateIDs)
				if err != nil {
					log.Error("Error deleting duplicates", "error", err, "count", len(duplicateIDs))
				} else {
					totalDeleted += len(duplicateIDs)
					log.Info("Successfully deleted duplicates", "count", len(duplicateIDs), "total_deleted", totalDeleted)
				}
			}

			// Update current index to this user's ID for next iteration
			currentIndex = user.ID

			// Small delay to prevent overwhelming the database
			// time.Sleep(10 * time.Millisecond)
		}
	}
}
