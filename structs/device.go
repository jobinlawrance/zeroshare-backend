package structs

import "github.com/google/uuid"

type Device struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	MachineName string    `gorm:"not null" json:"machine_name"`
	Platform    string    `gorm:"not null" json:"platform"`
	DeviceId    string    `gorm:"not null" json:"device_id"`
	IpAddress   string    `gorm:"null" json:"ip_address"`
	Created     int64     `gorm:"autoCreateTime"`
	Updated     int64     `gorm:"autoUpdateTime:milli" json:"updated"`
	UserId      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	User        User      `gorm:"foreignKey:UserId;references:ID"`
}
