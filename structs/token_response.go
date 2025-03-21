package structs

type TokenResponse struct {
	AuthToken   string `json:"auth_token"`
	RefresToken string `json:"refresh_token"`
}

type GoogleTokenResponse struct {
	Token string `json:"token"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}
