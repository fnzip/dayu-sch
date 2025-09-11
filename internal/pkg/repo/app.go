package repo

import (
	"context"

	"time"

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
}

// Run aggregation to update AppStats collection with user info
func (r *AppRepo) AggregateAppStats(ctx context.Context) error {
	// Compute start of today 00:01:00 in GMT+7
	now := time.Now().In(time.FixedZone("GMT+7", 7*60*60))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 1, 0, 0, now.Location())

	dateStr := today.Format("02-01-2006") // e.g. 11-09-2025

	pipeline := mongo.Pipeline{
		{{Key: "$lookup", Value: bson.M{
			"from": CollectionUsers,
			"let": bson.M{
				"appCode":    "$app_code",
				"minBalance": "$game_min_balance",
				"maxBalance": "$game_max_balance",
			},
			"pipeline": mongo.Pipeline{
				{{Key: "$facet", Value: bson.M{
					"latest_users_check_list": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$app_code", "$$appCode"},
							},
						}}},
						{{Key: "$sort", Value: bson.M{"last_check_at": -1}}},
						{{Key: "$project", Value: bson.M{
							"username":      1,
							"balance":       1,
							"coin":          1,
							"last_check_at": 1,
							"inc_balance":   1,
							"inc_coin":      1,
						}}},
						{{Key: "$limit", Value: 10}},
					},
					"valid_users_count": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$app_code", "$$appCode"},
							},
							"is_invalid_cred": false,
						}}},
						{{Key: "$count", Value: "count"}},
					},
					"playable_users_list": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$and": bson.A{
									bson.M{"$eq": bson.A{"$app_code", "$$appCode"}},
									bson.M{"$eq": bson.A{"$is_invalid_cred", false}},
									bson.M{
										"$and": bson.A{
											bson.M{"$gte": bson.A{"$balance", "$$minBalance"}},
											bson.M{"$lte": bson.A{"$balance", "$$maxBalance"}},
										},
									},
								},
							},
						}}},
						{{Key: "$sort", Value: bson.M{"last_check_at": -1}}},
						{{Key: "$project", Value: bson.M{
							"username":      1,
							"balance":       1,
							"coin":          1,
							"last_check_at": 1,
							"inc_balance":   1,
							"inc_coin":      1,
						}}},
						{{Key: "$limit", Value: 10}},
					},
					"playable_users_count": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$and": bson.A{
									bson.M{"$eq": bson.A{"$app_code", "$$appCode"}},
									bson.M{"$eq": bson.A{"$is_invalid_cred", false}},
									bson.M{
										"$and": bson.A{
											bson.M{"$gte": bson.A{"$balance", "$$minBalance"}},
											bson.M{"$lte": bson.A{"$balance", "$$maxBalance"}},
										},
									},
								},
							},
						}}},
						{{Key: "$count", Value: "count"}},
					},
					"jackpot_users_list": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$app_code", "$$appCode"},
							},
							"is_invalid_cred": false,
							"balance":         bson.M{"$gte": 100000},
						}}},
						{{Key: "$sort", Value: bson.M{"last_check_at": -1}}},
						{{Key: "$project", Value: bson.M{
							"username":      1,
							"balance":       1,
							"coin":          1,
							"last_check_at": 1,
							"inc_balance":   1,
							"inc_coin":      1,
						}}},
						{{Key: "$limit", Value: 10}},
					},
					"jackpot_users_count": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$app_code", "$$appCode"},
							},
							"is_invalid_cred": false,
							"balance":         bson.M{"$gte": 100000},
						}}},
						{{Key: "$count", Value: "count"}},
					},
					"processed_users_count": mongo.Pipeline{
						{{Key: "$match", Value: bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$app_code", "$$appCode"},
							},
							"is_invalid_cred": false,
							"last_check_at":   bson.M{"$gte": today},
						}}},
						{{Key: "$count", Value: "count"}},
					},
				}}},
			},
			"as": "users_info",
		}}},
		{{Key: "$addFields", Value: bson.M{
			"users_info": bson.M{
				"$arrayElemAt": bson.A{
					"$users_info",
					0,
				},
			},
		}}},
		{{Key: "$project", Value: bson.M{
			"_id":                     0,
			"app_code":                1,
			"game_min_balance":        1,
			"game_max_balance":        1,
			"latest_users_check_list": "$users_info.latest_users_check_list",
			"valid_users_count": bson.M{
				"$ifNull": bson.A{
					bson.M{
						"$arrayElemAt": bson.A{
							"$users_info.valid_users_count.count",
							0,
						},
					},
					0,
				},
			},
			"playable_users_list": "$users_info.playable_users_list",
			"playable_users_count": bson.M{
				"$ifNull": bson.A{
					bson.M{
						"$arrayElemAt": bson.A{
							"$users_info.playable_users_count.count",
							0,
						},
					},
					0,
				},
			},
			"jackpot_users_list": "$users_info.jackpot_users_list",
			"jackpot_users_count": bson.M{
				"$ifNull": bson.A{
					bson.M{
						"$arrayElemAt": bson.A{
							"$users_info.jackpot_users_count.count",
							0,
						},
					},
					0,
				},
			},
			"processed_users_count": bson.M{
				"$ifNull": bson.A{
					bson.M{
						"$arrayElemAt": bson.A{
							"$users_info.processed_users_count.count",
							0,
						},
					},
					0,
				},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"date": dateStr,
			// "first_valid_users_count":     "$valid_users_count",
			// "first_playable_users_count":  "$playable_users_count",
			// "first_jackpot_users_count":   "$jackpot_users_count",
			// "first_processed_users_count": "$processed_users_count",
			// "inc_valid_users_count":       0,
			// "inc_playable_users_count":    0,
			// "inc_jackpot_users_count":     0,
			// "inc_processed_users_count":   0,
		}}},
		{{Key: "$merge", Value: bson.M{
			"into": CollectionAppStats,
			"on": bson.A{
				"app_code",
				"date",
			},
			"whenMatched": bson.A{
				bson.M{
					"$set": bson.M{
						"first_valid_users_count": bson.M{
							"$ifNull": bson.A{
								"$first_valid_users_count",
								"$valid_users_count",
							},
						},
						"first_playable_users_count": bson.M{
							"$ifNull": bson.A{
								"$first_playable_users_count",
								"$playable_users_count",
							},
						},
						"first_jackpot_users_count": bson.M{
							"$ifNull": bson.A{
								"$first_jackpot_users_count",
								"$jackpot_users_count",
							},
						},
						"first_processed_users_count": bson.M{
							"$ifNull": bson.A{
								"$first_processed_users_count",
								"$processed_users_count",
							},
						},
						"inc_valid_users_count": bson.M{
							"$cond": bson.A{
								bson.M{
									"$ifNull": bson.A{
										"$first_valid_users_count",
										false,
									},
								},
								bson.M{
									"$subtract": bson.A{
										"$valid_users_count",
										"$first_valid_users_count",
									},
								},
								0,
							},
						},
						"inc_playable_users_count": bson.M{
							"$cond": bson.A{
								bson.M{
									"$ifNull": bson.A{
										"$first_playable_users_count",
										false,
									},
								},
								bson.M{
									"$subtract": bson.A{
										"$playable_users_count",
										"$first_playable_users_count",
									},
								},
								0,
							},
						},
						"inc_jackpot_users_count": bson.M{
							"$cond": bson.A{
								bson.M{
									"$ifNull": bson.A{
										"$first_jackpot_users_count",
										false,
									},
								},
								bson.M{
									"$subtract": bson.A{
										"$jackpot_users_count",
										"$first_jackpot_users_count",
									},
								},
								0,
							},
						},
						"inc_processed_users_count": bson.M{
							"$cond": bson.A{
								bson.M{
									"$ifNull": bson.A{
										"$first_processed_users_count",
										false,
									},
								},
								bson.M{
									"$subtract": bson.A{
										"$processed_users_count",
										"$$first_processed_users_count",
									},
								},
								0,
							},
						},
					},
				},
			},
			"whenNotMatched": "insert",
		}}},
	}

	cursor, err := r.mc.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return nil
}
