package structs

import "github.com/google/uuid"

type Peer struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"` // Auto-generated UUID
	MachineName string    `gorm:"not null" json:"machine_name"`
	NetworkId   string    `gorm:"not null" json:"network_id"`
	NodeId      string    `gorm:"not null" json:"node_id"`
	UserId      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Platform    string    `gorm:"null" json:"platform"`
	IpAddress   string    `gorm:"null" json:"ip_address"`
	Created     int64     `gorm:"autoCreateTime"`
	Updated     int64     `gorm:"autoUpdateTime:milli"`

	User User `gorm:"foreignKey:UserId;references:ID"`
}
