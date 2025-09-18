package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (r *UserRepo) GetClaimUsers(ctx context.Context, apps []*ModelApp, limit uint, index primitive.ObjectID) ([]*ModelUser, error) {
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
			"_id":             bson.M{"$gte": index},
			"app_code":        bson.M{"$in": appCodes},
			"is_invalid_cred": false,
			"last_check_at":   bson.M{"$lte": cutoffTime},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
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

// GetFirstUserFromIndex gets the first user starting from the given index
func (r *UserRepo) GetFirstUserFromIndex(ctx context.Context, index primitive.ObjectID) (*ModelUser, error) {
	filter := bson.M{"_id": bson.M{"$gt": index}}

	var user ModelUser
	err := r.mc.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// DeleteByID deletes a user by their _id
func (r *UserRepo) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}

	_, err := r.mc.DeleteOne(ctx, filter)
	return err
}

// FindDuplicate finds duplicate users by app_code and username
// Returns array of _id strings for duplicates (excluding currentId)
func (r *UserRepo) FindDuplicate(ctx context.Context, index primitive.ObjectID, appCode, username string, currentId primitive.ObjectID) ([]string, error) {
	pipeline := mongo.Pipeline{
		// Match documents with the specified app_code and username, _id greater than index, and _id not equal to currentId
		{{Key: "$match", Value: bson.M{
			"_id":      bson.M{"$gt": index, "$ne": currentId},
			"app_code": appCode,
			"username": username,
		}}},
		// Sort by _id to maintain order
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		// Group by app_code and username, collect all _ids
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "app_code", Value: "$app_code"},
				{Key: "username", Value: "$username"},
			}},
			{Key: "ids", Value: bson.D{{Key: "$push", Value: "$_id"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		// Only keep groups with more than 1 document (duplicates)
		{{Key: "$match", Value: bson.M{"count": bson.M{"$gt": 1}}}},
		// Remove the first _id from each group (keep the original, mark others as duplicates)
		{{Key: "$project", Value: bson.D{
			{Key: "duplicates", Value: bson.D{
				{Key: "$slice", Value: bson.A{"$ids", 1, bson.D{{Key: "$subtract", Value: bson.A{"$count", 1}}}}},
			}},
		}}},
	}

	cursor, err := r.mc.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var duplicateIDs []string
	for cursor.Next(ctx) {
		var result struct {
			Duplicates []primitive.ObjectID `bson:"duplicates"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		for _, id := range result.Duplicates {
			duplicateIDs = append(duplicateIDs, id.Hex())
		}
	}

	return duplicateIDs, nil
}

// DeleteBulk deletes multiple users by their _id array
func (r *UserRepo) DeleteBulk(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var objectIDs []primitive.ObjectID
	for _, id := range ids {
		objID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objectIDs = append(objectIDs, objID)
	}

	filter := bson.M{"_id": bson.M{"$in": objectIDs}}

	_, err := r.mc.DeleteMany(ctx, filter)
	return err
}
