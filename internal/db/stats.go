package db

import (
	"fmt"
	"time"
)

// StudyStats contains statistics about user's learning activity
type StudyStats struct {
	// Today's statistics
	CardsStudiedToday  int `json:"cards_studied_today"`
	AvgTimePerCardMs   int `json:"avg_time_per_card_ms"`
	TotalTimeStudiedMs int `json:"total_time_studied_ms"`
	NewCardsToday      int `json:"new_cards_today"`
	ReviewCardsToday   int `json:"review_cards_today"`

	// Overall statistics
	TotalCards       int    `json:"total_cards"`
	TotalTimeStudied string `json:"total_time_studied"` // Formatted as hh:mm:ss
	StudyDays        int    `json:"study_days"`         // Number of days with activity
	TotalReviews     int    `json:"total_reviews"`

	// Trends - could expand later
	StreakDays int `json:"streak_days"`
}

// StudyHistoryItem represents study activity for a single day
type StudyHistoryItem struct {
	Date        string `json:"date"`         // Format: "YYYY-MM-DD"
	CardCount   int    `json:"card_count"`   // Number of cards studied on this day
	TimeSpentMs int    `json:"time_spent_ms"` // Time spent studying on this day in milliseconds
}

// GetUserStudyStats retrieves study statistics for a user
func (s *Storage) GetUserStudyStats(userID string) (StudyStats, error) {
	stats := StudyStats{}

	// Get today's date boundaries in user's local timezone (using UTC for simplicity)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayEnd := todayStart.Add(24 * time.Hour)

	// Query 1: Get today's review count and time spent
	todayStatsQuery := `
		SELECT 
			COUNT(*) as count,
			IFNULL(SUM(r.time_spent_ms), 0) as total_time,
			IFNULL(AVG(r.time_spent_ms), 0) as avg_time
		FROM reviews r
		JOIN cards c ON c.id = r.card_id AND c.user_id = r.user_id
		WHERE r.user_id = ? 
		AND c.deleted_at IS NULL
		AND r.reviewed_at >= ? 
		AND r.reviewed_at < ?
	`

	var todayCount int
	var totalTimeMs int
	var avgTimeMs float64

	err := s.db.QueryRow(todayStatsQuery, userID, todayStart, todayEnd).Scan(&todayCount, &totalTimeMs, &avgTimeMs)
	if err != nil {
		return stats, err
	}

	stats.CardsStudiedToday = todayCount
	stats.TotalTimeStudiedMs = totalTimeMs
	stats.AvgTimePerCardMs = int(avgTimeMs)

	// Query 2: Get new cards vs review cards today
	stateStatsQuery := `
		SELECT
			SUM(CASE WHEN (c.state = 'new' AND r.reviewed_at IS NOT NULL) OR c.first_reviewed_at >= ? THEN 1 ELSE 0 END) as new_cards,
			SUM(CASE WHEN c.state IN ('review', 'learning', 'relearning') AND c.first_reviewed_at < ? THEN 1 ELSE 0 END) as review_cards
		FROM cards c
		JOIN reviews r ON c.id = r.card_id AND c.user_id = r.user_id
		WHERE c.user_id = ? 
		AND r.reviewed_at >= ? 
		AND r.reviewed_at < ?
		AND c.deleted_at IS NULL
		GROUP BY c.user_id
	`

	err = s.db.QueryRow(stateStatsQuery, todayStart, todayStart, userID, todayStart, todayEnd).Scan(&stats.NewCardsToday, &stats.ReviewCardsToday)
	if err != nil {
		// If no results, just set to 0 (already initialized to 0)
		stats.NewCardsToday = 0
		stats.ReviewCardsToday = 0
	}

	// Query 3: Get total stats
	totalStatsQuery := `
		SELECT 
			COUNT(DISTINCT c.id) as total_cards,
			COUNT(*) as total_reviews,
			IFNULL(SUM(r.time_spent_ms), 0) as total_time
		FROM cards c
		LEFT JOIN reviews r ON c.id = r.card_id AND c.user_id = r.user_id
		WHERE c.user_id = ? AND c.deleted_at IS NULL
	`

	var totalTimeAllMs int
	err = s.db.QueryRow(totalStatsQuery, userID).Scan(&stats.TotalCards, &stats.TotalReviews, &totalTimeAllMs)
	if err != nil {
		return stats, err
	}

	// Format total time studied correctly
	totalDuration := time.Duration(totalTimeAllMs) * time.Millisecond
	hours := int(totalDuration.Hours())
	minutes := int(totalDuration.Minutes()) % 60
	seconds := int(totalDuration.Seconds()) % 60
	stats.TotalTimeStudied = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	// Query 4: Get number of days with activity
	daysQuery := `
		SELECT 
			COUNT(DISTINCT DATE(reviewed_at)) as study_days
		FROM reviews
		JOIN cards ON cards.id = reviews.card_id AND cards.user_id = reviews.user_id
		WHERE cards.user_id = ?
		AND cards.deleted_at IS NULL
	`

	err = s.db.QueryRow(daysQuery, userID).Scan(&stats.StudyDays)
	if err != nil {
		stats.StudyDays = 0
	}

	// For now, set streak to 0 (can implement streak calculation later)
	stats.StreakDays = 0

	return stats, nil
}

// GetUserStudyHistory retrieves study history for a user for the last N days
func (s *Storage) GetUserStudyHistory(userID string, days int) ([]StudyHistoryItem, error) {
	history := []StudyHistoryItem{}

	// Default to 100 days if not specified
	if days <= 0 {
		days = 100
	}

	// Calculate the start date (N days ago)
	now := time.Now()
	startDate := now.AddDate(0, 0, -days)
	
	// Format dates
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := now.Format("2006-01-02")

	// Query to get daily activity
	query := `
		SELECT 
			DATE(r.reviewed_at) as study_date,
			COUNT(*) as card_count,
			SUM(r.time_spent_ms) as time_spent_ms
		FROM reviews r
		JOIN cards c ON c.id = r.card_id AND c.user_id = r.user_id
		WHERE r.user_id = ? 
		AND c.deleted_at IS NULL
		AND DATE(r.reviewed_at) >= DATE(?)
		AND DATE(r.reviewed_at) <= DATE(?)
		GROUP BY DATE(r.reviewed_at)
		ORDER BY DATE(r.reviewed_at) ASC
	`

	rows, err := s.db.Query(query, userID, startDateStr, endDateStr)
	if err != nil {
		return history, err
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		var item StudyHistoryItem
		var dateStr string
		
		if err := rows.Scan(&dateStr, &item.CardCount, &item.TimeSpentMs); err != nil {
			return history, err
		}
		
		item.Date = dateStr
		history = append(history, item)
	}

	if err = rows.Err(); err != nil {
		return history, err
	}

	return history, nil
}
