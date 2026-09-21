package httpapi

import (
	"fmt"
	"math"
	"time"
)

const hoursPerDay = 24

// relativeTime bildet moment().from() in der englischen Lokalisierung nach,
// die das Node-Original fuer das Feld process-start verwendet hat. Die
// Bedingungskette spiegelt moment's relativeTime wider und vergleicht den
// abgerundeten Wert der naechst groesseren Einheit.
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

	exactDays := d.Hours() / hoursPerDay

	seconds := int(math.Round(d.Seconds()))
	minutes := int(math.Round(d.Minutes()))
	hours := int(math.Round(d.Hours()))
	days := int(math.Round(exactDays))
	months := int(math.Round(exactDays / daysPerMonth))
	years := int(math.Round(exactDays / daysPerYear))

	switch {
	case seconds <= 44:
		return "a few seconds ago"
	case minutes <= 1:
		return "a minute ago"
	case minutes < 45:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours <= 1:
		return "an hour ago"
	case hours < 22:
		return fmt.Sprintf("%d hours ago", hours)
	case days <= 1:
		return "a day ago"
	case days < 26:
		return fmt.Sprintf("%d days ago", days)
	case months <= 1:
		return "a month ago"
	case months < 11:
		return fmt.Sprintf("%d months ago", months)
	case years <= 1:
		return "a year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}
