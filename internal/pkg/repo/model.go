package repo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ModelUser struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	AppCode  string             `bson:"app_code" json:"app_code"`
	Username string             `bson:"username" json:"username"`
	// Password      string             `bson:"password" json:"password"`
	IsInvalidCred bool      `bson:"is_invalid_cred" json:"is_invalid_cred"`
	Coin          float64   `bson:"coin" json:"coin"`
	Balance       float64   `bson:"balance" json:"balance"`
	BalanceWD     float64   `bson:"balance_wd" json:"balance_wd"`
	LastCheckAt   time.Time `bson:"last_check_at" json:"last_check_at"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	// ProxyID       primitive.ObjectID `bson:"proxy_id" json:"proxy_id"`
	SID string `bson:"sid,omitempty" json:"sid,omitempty"`
	// IsBot         bool               `bson:"is_bot,omitempty" json:"is_bot,omitempty"`
	AppDetail ModelApp `bson:"app_detail,omitempty" json:"app_detail,omitempty"`
}

type ModelApp struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IsActive           bool               `bson:"is_active" json:"is_active"`
	AppCode            string             `bson:"app_code" json:"app_code"`
	WalletURL          string             `bson:"wallet_url" json:"wallet_url"`
	WalletCustomInfoID string             `bson:"wallet_custom_info_id" json:"wallet_custom_info_id"`
	VipActivityId      string             `bson:"vip_activity_id" json:"vip_activity_id"`
	GameApiID          string             `bson:"game_api_id" json:"game_api_id"`
	GameID             string             `bson:"game_id" json:"game_id"`
	GameSymbol         string             `bson:"game_symbol" json:"game_symbol"`
	// WalletThreads      uint               `bson:"wallet_threads" json:"wallet_threads"`
	// GameThreads        uint               `bson:"game_threads" json:"game_threads"`
	// GameMinBalance     float64            `bson:"game_min_balance" json:"game_min_balance"`
	// GameMaxBalance     float64            `bson:"game_max_balance" json:"game_max_balance"`
	// GameMinStop        float64            `bson:"game_min_stop" json:"game_min_stop"`
	// GameMaxStop        float64            `bson:"game_max_stop" json:"game_max_stop"`
	// GamePlayStart      int                `bson:"game_play_start" json:"game_play_start"`
	// GamePlayStop       int                `bson:"game_play_stop" json:"game_play_stop"`
	// Services           ServicesList       `bson:"services" json:"services"`
}
