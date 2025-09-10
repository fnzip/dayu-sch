package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepo struct {
	md *mongo.Database
	mc *mongo.Collection
}

func NewUserRepo(md *mongo.Database) *UserRepo {
	mc := md.Collection(CollectionUsers)

	return &UserRepo{
		md: md,
		mc: mc,
	}
}

func (r *UserRepo) GetClaimUsers(ctx context.Context, apps []*ModelApp, limit uint) ([]*ModelUser, error) {
	// Calculate previous day at 23:59:00 GMT+7
	loc, _ := time.LoadLocation("Asia/Jakarta") // GMT+7
	now := time.Now().In(loc)
	prevDay := now.AddDate(0, 0, -1)
	cutoffTime := time.Date(prevDay.Year(), prevDay.Month(), prevDay.Day(), 23, 59, 0, 0, loc)

	var appCodes []string

	for _, app := range apps {
		appCodes = append(appCodes, app.AppCode)
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"app_code":        bson.M{"$in": appCodes},
			"is_invalid_cred": false,
			"last_check_at":   bson.M{"$lte": cutoffTime},
		}}},
		{{Key: "$limit", Value: limit}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: CollectionApps},
			{Key: "localField", Value: "app_code"},
			{Key: "foreignField", Value: "app_code"},
			{Key: "as", Value: "app_detail"},
		}}},
		bson.D{{Key: "$addFields", Value: bson.D{
			{Key: "app_detail", Value: bson.D{
				{Key: "$arrayElemAt", Value: bson.A{"$app_detail", 0}},
			}},
		}}},
	}

	cursor, err := r.mc.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*ModelUser
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}
