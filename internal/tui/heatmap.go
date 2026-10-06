package tui

import "time"

// Heatmap draws solves per day as a 7-row grid (Monday first), one column per week, oldest week
// left and the current week last. Each cell is two runes wide; days after today are blank.
func Heatmap(days map[string]int, now time.Time, weeks int) []string {
	if weeks < 1 {
		return nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	start := monday.AddDate(0, 0, -7*(weeks-1))
	rows := make([]string, 7)
	for w := 0; w < weeks; w++ {
		for d := 0; d < 7; d++ {
			day := start.AddDate(0, 0, 7*w+d)
			switch n := days[day.Format("2006-01-02")]; {
			case day.After(today):
				rows[d] += "  "
			case n == 0:
				rows[d] += "· "
			case n == 1:
				rows[d] += "▒ "
			case n < 4:
				rows[d] += "▓ "
			default:
				rows[d] += "█ "
			}
		}
	}
	return rows
}
