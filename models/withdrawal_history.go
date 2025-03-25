package models

import (
	"time"

	"gorm.io/gorm"
)

type WithdrawalHistory struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	UserID          uint           `json:"user_id"`
	User            User           `gorm:"foreignKey:UserID" json:"-"`
	Amount          int64          `json:"amount"`
	Status          int            `json:"status"` // 0: Pending, 1: Rejected, 2: Completed
	Note            string         `json:"note"`
	TransactionCode string         `json:"transaction_code"`
	BankID          uint           `json:"bank_id"`
	Bank            Bank           `gorm:"foreignKey:BankID;references:BankId" json:"-"`
}
