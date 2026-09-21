package httpapi

import (
	"fmt"
	"math"
	"time"
)

const hoursPerDay = 24

// relativeTime bildet moment().from() in der englischen Lokalisierung nach,
// die das Node-Original fuer das Feld process-start verwendet hat. Die
// Schwellwerte stammen aus momentjs (relativeTime thresholds).
//
// momentjs rechnet Tage in Monate um ueber daysToMonths: 400 Jahre haben
// 146097 Tage und 4800 Monate. Ein flacher 30-Tage-Monat waerae davon in
// der oberen Haelfte des Monats-Bereichs um einen ganzen Monat ab.
const (
	daysPerMonth = 146097.0 / 4800.0 // 30.436875
	daysPerYear  = daysPerMonth * 12 // 365.2425
)

func relativeTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	seconds := int(math.Round(d.Seconds()))
	minutes := int(math.Round(d.Minutes()))
	hours := int(math.Round(d.Hours()))
	days := int(math.Round(d.Hours() / hoursPerDay))
	months := int(math.Round(float64(days) / daysPerMonth))
	years := int(math.Round(float64(days) / daysPerYear))

	switch {
	case seconds < 45:
		return "a few seconds ago"
	case seconds < 90:
		return "a minute ago"
	case minutes < 45:
		return fmt.Sprintf("%d minutes ago", minutes)
	case minutes < 90:
		return "an hour ago"
	case hours < 22:
		return fmt.Sprintf("%d hours ago", hours)
	case hours < 36:
		return "a day ago"
	case days < 26:
		return fmt.Sprintf("%d days ago", days)
	case days < 46:
		return "a month ago"
	case days < 320:
		return fmt.Sprintf("%d months ago", months)
	case days < 548:
		return "a year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}
