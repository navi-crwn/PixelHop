package i18n

import "time"

// Dur writes a length of time in the language: "10 minutes", "1 day".
func (l Locale) Dur(d time.Duration) string {
	var n int
	var unit string
	switch {
	case d < time.Hour:
		n, unit = int(d.Minutes()), "minute"
	case d < 48*time.Hour:
		n, unit = int(d.Hours()), "hour"
	case d < 60*24*time.Hour:
		n, unit = int(d.Hours()/24), "day"
	case d < 730*24*time.Hour:
		n, unit = int(d.Hours()/24/30), "month"
	default:
		n, unit = int(d.Hours()/24/365), "year"
	}
	n = max(n, 1)
	form := "other"
	if n == 1 {
		form = "one"
	}
	return l.T("time."+unit+"."+form, "n", n)
}

// Ago says how long ago a Unix time was: "3 hours ago", "just now", "never".
func (l Locale) Ago(t int64) string {
	if t == 0 {
		return l.T("time.never")
	}
	d := time.Since(time.Unix(t, 0))
	if d < time.Minute {
		return l.T("time.just_now")
	}
	return l.T("time.ago", "dur", l.Dur(d))
}

// Until says how long remains until a Unix time: "2 days".
func (l Locale) Until(t int64) string {
	return l.Dur(time.Until(time.Unix(t, 0)))
}
