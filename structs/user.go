package structs

import "github.com/google/uuid"

type User struct {
	ID            uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	GoogleID      string    `gorm:"unique;not null" json:"id"`
	Email         string    `gorm:"unique;not null" json:"email"`
	FamilyName    string    `gorm:"not null" json:"family_name"`
	GivenName     string    `gorm:"not null" json:"given_name"`
	Locale        string    `json:"locale"`
	Name          string    `gorm:"not null" json:"name"`
	Picture       string    `json:"picture"`
	VerifiedEmail bool      `gorm:"not null" json:"verified_email"`
}
