package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AppRepo struct {
	md *mongo.Database
	mc *mongo.Collection
}

func NewAppRepo(md *mongo.Database) *AppRepo {
	mc := md.Collection(CollectionApps)

	return &AppRepo{
		md: md,
		mc: mc,
	}
}

func (r *AppRepo) GetClaimAppCodes(ctx context.Context) ([]*ModelApp, error) {
	filter := bson.M{
		"is_active":      true,
		"services.claim": true,
	}

	cur, err := r.mc.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var apps []*ModelApp

	if err = cur.All(ctx, &apps); err != nil {
		return nil, err
	}

	return apps, nil

	// for cur.Next(ctx) {
	// 	var app bson.M
	// 	if err := cur.Decode(&app); err == nil {
	// 		if code, ok := app["app_code"].(string); ok {
	// 			codes = append(codes, code)
	// 		}
	// 	}
	// }

	// return codes, nil
}
