package cfbatch

type CFBatchUser struct {
	// user
	ID       string  `json:"_id"`
	AppCode  string  `json:"app_code"`
	Username string  `json:"username"`
	Coin     float64 `json:"coin"`
	Balance  float64 `json:"balance"`
	// app
	WalletURL          string `json:"wallet_url"`
	WalletCustomInfoID string `json:"wallet_custom_info_id"`
	VipActivityId      string `json:"vip_activity_id"`
}

type CFBatchUserBody struct {
	Users []CFBatchUser `json:"users"`
}
