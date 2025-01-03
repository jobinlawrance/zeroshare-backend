package structs

type TokenResponse struct {
	AuthToken   string `json:"auth_token"`
	RefresToken string `json:"refresh_token"`
	ZtNetworkId string `json:"zt_network_id"`
}

type GoogleTokenResponse struct {
	Token string `json:"token"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}