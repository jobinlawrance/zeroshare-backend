package structs

type TokenResponse struct {
	AuthToken   string `json:"auth_token"`
	RefresToken string `json:"refresh_token"`
}