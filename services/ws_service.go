package services

import (
	"fmt"
	"log"
	"time"

	"new/config"
	"new/models"

	"github.com/olahol/melody"
	"gorm.io/gorm"
)

func UpdateDailyRevenue() error {
	query := `
		INSERT INTO user_revenues (user_id, date, revenue, order_count, created_at, updated_at)
		SELECT 
			invoices.admin_id AS user_id,
			DATE(orders.created_at) AS date,
			SUM(invoices.total_amount) AS revenue,
			COUNT(invoices.id) AS order_count,
			NOW() AS created_at,
			NOW() AS updated_at
		FROM invoices
		JOIN orders ON invoices.order_id = orders.id
		GROUP BY invoices.admin_id, DATE(orders.created_at)
		ON CONFLICT (user_id, date)
		DO UPDATE SET 
			revenue = EXCLUDED.revenue,
			order_count = EXCLUDED.order_count,
			updated_at = NOW();
	`

	if err := config.DB.Exec(query).Error; err != nil {
		log.Printf("❌ Lỗi cập nhật doanh thu hàng ngày: %v", err)
		return fmt.Errorf("updateDailyRevenue error: %w", err)
	}

	log.Printf("✅ Cập nhật doanh thu hàng ngày thành công lúc %v", time.Now())
	return nil
}

// GetTodayUserRevenue lấy danh sách doanh thu trong ngày hôm nay
func GetTodayUserRevenue() ([]models.UserRevenue, error) {
	var revenues []models.UserRevenue
	today := time.Now().Format("2006-01-02")

	err := config.DB.Where("date = ?", today).Find(&revenues).Error
	if err != nil {
		return nil, fmt.Errorf("❌ Lỗi khi truy vấn doanh thu ngày hiện tại: %w", err)
	}

	return revenues, nil
}

// UpdateUserAmounts cập nhật amount của user dựa trên revenue hôm nay
func UpdateUserAmounts(m *melody.Melody) error {
	db := config.DB

	revenues, err := GetTodayUserRevenue()
	if err != nil {
		log.Println("❌ Lỗi lấy doanh thu:", err)
		return err
	}

	if len(revenues) == 0 {
		log.Println("ℹ️ Không có doanh thu nào để cập nhật hôm nay.")
		return nil
	}

	// Bắt đầu transaction
	tx := db.Begin()

	for _, rev := range revenues {
		if err := tx.Model(&models.User{}).
			Where("id = ?", rev.UserID).
			Update("amount", gorm.Expr("amount + ?", rev.Revenue)).Error; err != nil {
			tx.Rollback() // Nếu có lỗi, rollback transaction
			log.Printf("❌ Lỗi cập nhật amount cho user %d: %v\n", rev.UserID, err)
			return err
		}

		log.Printf("✅ Cập nhật thành công user_id %d: +%.2f\n", rev.UserID, rev.Revenue)

		//thông báo
		message := fmt.Sprintf("🔔 User %d đã được cộng %.2f vào tài khoản.", rev.UserID, rev.Revenue)
		m.Broadcast([]byte(message))
	}

	tx.Commit()

	log.Println("✅ Hoàn tất cập nhật amount cho tất cả users.")
	return nil
}
