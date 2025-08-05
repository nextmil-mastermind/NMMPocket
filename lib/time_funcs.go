package lib

import "time"

// convertToEasternTime manually converts a UTC time to Eastern Time (EST/EDT)
func convertToEasternTime(utcTime time.Time) time.Time {
	year := utcTime.Year()

	// Calculate DST start (second Sunday of March)
	dstStart := time.Date(year, time.March, 1, 2, 0, 0, 0, time.UTC)
	dstStart = dstStart.AddDate(0, 0, (14-int(dstStart.Weekday()))%7) // Second Sunday

	// Calculate DST end (first Sunday of November)
	dstEnd := time.Date(year, time.November, 1, 2, 0, 0, 0, time.UTC)
	dstEnd = dstEnd.AddDate(0, 0, (7-int(dstEnd.Weekday()))%7) // First Sunday

	// Determine if we're in daylight saving time
	isDST := utcTime.After(dstStart) && utcTime.Before(dstEnd)

	if isDST {
		// EDT: UTC-4
		return utcTime.Add(-4 * time.Hour)
	} else {
		// EST: UTC-5
		return utcTime.Add(-5 * time.Hour)
	}
}

// isDaylightSavingTime checks if the given UTC time falls within daylight saving time
func isDaylightSavingTime(utcTime time.Time) bool {
	year := utcTime.Year()

	// Calculate DST start (second Sunday of March)
	dstStart := time.Date(year, time.March, 1, 2, 0, 0, 0, time.UTC)
	dstStart = dstStart.AddDate(0, 0, (14-int(dstStart.Weekday()))%7) // Second Sunday

	// Calculate DST end (first Sunday of November)
	dstEnd := time.Date(year, time.November, 1, 2, 0, 0, 0, time.UTC)
	dstEnd = dstEnd.AddDate(0, 0, (7-int(dstEnd.Weekday()))%7) // First Sunday

	return utcTime.After(dstStart) && utcTime.Before(dstEnd)
}

// formatEasternTime converts UTC time to Eastern time and formats it with proper timezone abbreviation
func formatEasternTime(utcTime time.Time) string {
	easternTime := convertToEasternTime(utcTime)

	if isDaylightSavingTime(utcTime) {
		return easternTime.Format("01/02/2006 03:04 PM") + " EDT"
	} else {
		return easternTime.Format("01/02/2006 03:04 PM") + " EST"
	}
}
